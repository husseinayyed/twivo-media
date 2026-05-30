#include "ImageType.hpp"
#include "TokenVerifier.hpp"
#include <cstddef>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <string>
#include <string_view>
#include <vector>
#include <iomanip>
#include <sstream>
#include <random>
#include <sw/redis++/redis++.h>
#include <uwebsockets/App.h>
#include <curl/curl.h>
#include <openssl/evp.h>

using namespace std;
using namespace sw::redis;

namespace fs = filesystem;

// Constants
constexpr short int PORT = 8020;
constexpr size_t MAX_UPLOAD_SIZE = 10 * 1024 * 1024; // 10 MB
constexpr int RATE_LIMIT_REQUESTS = 5; // Max requests per minute
constexpr uint32_t MAX_IMAGE_DIMENSION = 2000;

// Global Redis connection
static Redis redis(std::getenv("REDIS_URL"));
Redis* redisPtr = &redis;

// ==================== Helper Functions ====================

string readPublicKey() {
    const char* cert_path_env = std::getenv("SSL_CERT_PATH");
    string cert_path = (cert_path_env) ? cert_path_env : "/app/keys/public.pem";

    ifstream f(cert_path, ios::in | ios::binary);
    
    if (!f.is_open()) {
        cerr << "FATAL ERROR: Could not open public key file at: " << cert_path << endl;
        exit(1);
    }

    string key((istreambuf_iterator<char>(f)), istreambuf_iterator<char>());
    f.close();

    if (key.empty() || key.find("-----BEGIN") == string::npos) {
        cerr << "FATAL ERROR: Key file is empty or malformed" << endl;
        exit(1);
    }

    cout << "✅ Public key loaded: " << cert_path << " (" << key.length() << " bytes)" << endl;
    return key;
}

string calculateSHA256(const vector<uint8_t>& data) {
    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (!ctx) return "";
    
    unsigned char hash[EVP_MAX_MD_SIZE];
    unsigned int hash_len = 0;
    
    EVP_DigestInit_ex(ctx, EVP_sha256(), nullptr);
    EVP_DigestUpdate(ctx, data.data(), data.size());
    EVP_DigestFinal_ex(ctx, hash, &hash_len);
    
    EVP_MD_CTX_free(ctx);
    
    stringstream ss;
    for (unsigned int i = 0; i < hash_len; i++) {
        ss << hex << setw(2) << setfill('0') << (int)hash[i];
    }
    return ss.str();
}

string generateNanoId12() {
    const string alphabet = 
        "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz";
    const size_t length = 12;
    
    random_device rd;
    mt19937 generator(rd());
    uniform_int_distribution<> distribution(0, alphabet.size() - 1);
    
    string nanoId;
    nanoId.reserve(length);
    for (size_t i = 0; i < length; ++i) {
        nanoId += alphabet[distribution(generator)];
    }
    return nanoId;
}

bool rateLimit(auto* res) {
    const string ip = string(res->getRemoteAddressAsText());
    
    string script = R"(
        local val = redis.call('INCR', KEYS[1])
        if val == 1 then
            redis.call('EXPIRE', KEYS[1], 60)
        end
        if val > tonumber(ARGV[1]) then
            return 0
        else
            return 1
        end
    )";

    try {
        auto result = redis.eval<long long>(
            script, 
            {format("ip:limit:{}", ip)},
            {to_string(RATE_LIMIT_REQUESTS)}
        );
        
        if (!result) {
            res->writeStatus("429 Too Many Requests")->end("Too many requests");
            return true;
        }
    } catch (const sw::redis::ReplyError& e) {
        cerr << "Redis error: " << e.what() << "\n";
        return false;
    }
    
    return false;
}
auto getExtensionString(ImageType type) -> std::string {
    switch (type) {
        case ImageType::PNG:  return ".png";
        case ImageType::JPG:  return ".jpg";
        case ImageType::WEBP: return ".webp";
        default:              return "";
    }
}
bool uploadToSeaweed(const vector<unsigned char>& webpData, const string& filepath, const string& ext) {
    CURL* curl = curl_easy_init();
    if (!curl) return false;

    // Extract filename from path
    string filename = filepath.substr(filepath.find_last_of('/') + 1);
    
    // Build the URL
    string url = "http://weed-filer:8888" + filepath;
    
    // Create multipart form
    curl_mime* mime = curl_mime_init(curl);
    curl_mimepart* part = curl_mime_addpart(mime);
    
    // Add file data as a form field
    curl_mime_name(part, "file");
    curl_mime_data(part, reinterpret_cast<const char*>(webpData.data()), webpData.size());
    curl_mime_filename(part, filename.c_str());
    
    // Fix: Convert string to const char* using .c_str()
    string mimeType = "image/" + ext;
    curl_mime_type(part, mimeType.c_str());
    
    // Set up curl
    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_MIMEPOST, mime);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 30L);
    
    // Execute request
    CURLcode res = curl_easy_perform(curl);
    
    long responseCode = 0;
    if (res == CURLE_OK) {
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &responseCode);
        cout << "Upload response code: " << responseCode << endl;
    } else {
        cerr << "CURL error: " << curl_easy_strerror(res) << endl;
    }
    
    // Cleanup
    curl_mime_free(mime);
    curl_easy_cleanup(curl);
    
    return res == CURLE_OK && (responseCode == 200 || responseCode == 201);
}

string getOrientationString(const ImageDimensions& dim) {
    if (!dim.valid) return "unknown";
    if (dim.width > dim.height) return "horizontal";
    if (dim.height > dim.width) return "vertical";
    return "square";
}

// ==================== Upload Handler ====================

void handleUpload(auto* res, auto* req, const string& pubKeyString) {
    // Rate limiting
    if (rateLimit(res)) return;

    // Token validation
    auto token = req->getHeader("x-twivo-backend");
    if (token.empty()) {
        res->writeStatus("400 Bad Request")->end("Missing JWT token");
        return;
    }

    auto decodedOpt = verifyUploadImageGetUserId(token, pubKeyString, *redisPtr);
    if (!decodedOpt.has_value()) {
        res->writeStatus("401 Unauthorized")->end("Invalid or expired token");
        return;
    }

    const auto& decoded = decodedOpt.value();

    // Extract user claims
    if (!decoded.has_subject()) {
        res->writeStatus("401 Unauthorized")->end("Missing user identifier (sub)");
        return;
    }
    string userId = decoded.get_subject();

    if (!decoded.has_payload_claim("id")) {
        res->writeStatus("401 Unauthorized")->end("Missing id claim");
        return;
    }
    
    string twiId;
    try {
        twiId = decoded.get_payload_claim("id").as_string();
    } catch (const bad_cast& e) {
        cerr << "Invalid type for id claim: " << e.what() << "\n";
        res->writeStatus("401 Unauthorized")->end("Invalid id claim type");
        return;
    }

    // Shared state for async data processing
    auto totalSize = make_shared<size_t>(0);
    auto buffer = make_shared<vector<unsigned char>>();
    auto fileType = make_shared<ImageType>(ImageType::UNKNOWN);
    auto dimensionsChecked = make_shared<bool>(false);
    auto isCompleted = make_shared<bool>(false);

    res->onData([res, buffer, fileType, totalSize, userId, twiId, 
                 dimensionsChecked, isCompleted](string_view chunk, bool isLast) mutable {
        if (*isCompleted) return;

        *totalSize += chunk.size();

        if (*totalSize > MAX_UPLOAD_SIZE) {
            *isCompleted = true;
            res->writeStatus("413 Payload Too Large")->end("File too large");
            return;
        }

        buffer->insert(buffer->end(), chunk.begin(), chunk.end());

        // Detect image type
        if (buffer->size() >= 12 && *fileType == ImageType::UNKNOWN) {
            *fileType = getImageType(
                reinterpret_cast<const char*>(buffer->data()), 
                buffer->size()
            );
            
            if (*fileType == ImageType::UNKNOWN) {
                *isCompleted = true;
                res->writeStatus("400 Bad Request")->end("Invalid image format");
                return;
            }
        }

        // Validate dimensions when enough data is available
        if (!*dimensionsChecked && *fileType != ImageType::UNKNOWN) {
            bool readyToValidate = false;

            switch (*fileType) {
                case ImageType::PNG:
                    readyToValidate = (buffer->size() >= 24);
                    break;
                case ImageType::WEBP:
                    readyToValidate = (buffer->size() >= 30);
                    break;
                case ImageType::JPG:
                    for (size_t i = 2; i + 1 < buffer->size(); ++i) {
                        if (buffer->at(i) == 0xFF) {
                            unsigned char marker = buffer->at(i + 1);
                            if (marker == 0xC0 || marker == 0xC2) {
                                readyToValidate = true;
                                break;
                            }
                        }
                    }
                    if (buffer->size() > 65536) readyToValidate = true;
                    break;
                default:
                    break;
            }

            if (readyToValidate) {
                ImageDimensions dim = getImageDimensions(
                    span<const char>(
                        reinterpret_cast<const char*>(buffer->data()), 
                        buffer->size()
                    ), 
                    *fileType
                );

                if (dim.valid) {
                    if (dim.width > MAX_IMAGE_DIMENSION || dim.height > MAX_IMAGE_DIMENSION) {
                        *isCompleted = true;
                        res->writeStatus("422 Unprocessable Entity")
                           ->end("Image dimensions cannot exceed " + 
                                to_string(MAX_IMAGE_DIMENSION) + "x" + 
                                to_string(MAX_IMAGE_DIMENSION));
                        return;
                    }
                    *dimensionsChecked = true;
                }
            }
        }

        // Process completed upload
        if (isLast) {
            *isCompleted = true;

            if (*fileType == ImageType::UNKNOWN || !*dimensionsChecked) {
                res->writeStatus("400 Bad Request")->end("Invalid or truncated image");
                return;
            }

            // Generate content-addressed path
            const string sha256_hex = calculateSHA256(*buffer);
            const string nanoId = generateNanoId12();
            const string ext = getExtensionString(*fileType);
            const string filepath = "/i/" + sha256_hex + ext;

            // Upload to SeaweedFS
            if (!uploadToSeaweed(*buffer, filepath,ext)) {
                res->writeStatus("500 Internal Server Error")->end("Storage engine write fault");
                return;
            }

            // Get final dimensions
            ImageDimensions finalDim = getImageDimensions(
                span<const char>(
                    reinterpret_cast<const char*>(buffer->data()), 
                    buffer->size()
                ), 
                *fileType
            );
            string orientation_str = getOrientationString(finalDim);

            // Save metadata to Redis
            vector<pair<string, string>> fields = {
                {"id", twiId},
                {"user_id", userId},
                {"path", nanoId},
                {"sha256", sha256_hex},
                {"orientation", orientation_str}
            };

            redisPtr->xadd("uploads:stream", "*", fields.begin(), fields.end());
            res->writeStatus("201 Created")->end("Upload successful - Path: " + filepath);
        }
    });

    // Handle client disconnect
    res->onAborted([buffer, fileType, totalSize, userId, isCompleted]() {
        *isCompleted = true;
        buffer->clear();
        *totalSize = 0;
        *fileType = ImageType::UNKNOWN;
        cout << "⚠️ Connection dropped for user: " << userId << '\n';
    });
}

// ==================== Main ====================

auto main() -> int {
    string pubKey = readPublicKey();
    
    // Test Redis connection
    redis.set("ping", "pong");
    auto val = redis.get("ping");
    cout << *val << endl;

    uWS::App app;
    
    // Health check route
    app.get("/*", [](auto* res, auto* /*req*/) {
        if (rateLimit(res)) return;
        res->end("hello world!");
    });

    // Upload route
    app.post("/upload", [pubKeyString = string(pubKey)](auto* res, auto* req) {
        handleUpload(res, req, pubKeyString);
    });

    // Start server
    app.listen(PORT, [](auto* listen_socket) {
        if (listen_socket) {
            cout << "🚀 Server running at http://localhost:" << PORT << '\n';
        }
    }).run();

    return 0;
}