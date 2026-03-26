#include "ImageProcessor.hpp"
#include "ImageType.hpp"
#include "TokenVerifier.hpp"
#include "jwt-cpp/jwt.h"
#include "uuid_v4.h"
#include <cstddef>
#include <cstdlib>
#include <filesystem>
#include <format>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <random>
#include <string>
#include <string_view>
#include <sw/redis++/redis++.h>
#include <sw/redis++/redis.h>
#include <uwebsockets/App.h>

using namespace std;
using namespace sw::redis;
namespace fs = filesystem;

constexpr short int PORT = 8080;
constexpr short int magicbytes = 12;
constexpr size_t MAX_UPLOAD_SIZE =
    static_cast<size_t>(10 * 1024 * 1024); // 10 MB
static Redis redis("tcp://127.0.0.1:6379");
Redis *redisPtr = &redis;

auto readPublicKey() -> string {
  ifstream f("../twivo-backend.pem");
  if (!f.is_open()) {
    cerr << "ERROR: Cannot open twivo-backend.pem" << '\n';
    exit(1);
  }
  string key((istreambuf_iterator<char>(f)),
             istreambuf_iterator<char>());
  if (key.empty()) {
    cerr << "ERROR: Empty twivo-backend.pem" << '\n';
    exit(1);
  }
  f.close();
  return key;
}

auto rateLimit(auto res) -> bool {
  const string ip = string(res->getRemoteAddressAsText());
  const int limit = 5; // Maximum requests per minute

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
    auto result =
        redis.eval<long long>(script, {format("ip:limit:{}", ip)}, // KEYS
                              {to_string(limit)});            // ARGV
    if (!result) {
      res->writeStatus("429 Too Many Requests")->end("Too many requests");
      return true;
    }
  } catch (const sw::redis::ReplyError &e) {
    cerr << "Redis error: " << e.what() << "\n";
    // Allow the request if Redis fails (fail open) or block it (fail closed)
    // Here we'll allow it but log the error
    return false;
  }

  return false;
}

auto main() -> int {
  string pubKey = readPublicKey();
  uWS::App app;
  redis.set("ping", "pong");
  auto val = redis.get("ping");
  cout << *val;
  // Create UUID generator once
  UUIDv4::UUIDGenerator<mt19937_64> uuidGenerator;

  // Simple GET route
  app.get("/*", [](auto *res, auto * /*req*/) {
    if (rateLimit(res)) {
      return;
    };
    res->end("hello world!");
  });

  // Serve file route
  app.get("/media/*", [](auto *res, auto *req) {
    // Extract file path from URL (remove "/media/" prefix)
    string urlPath = string(req->getUrl());
    string filePath = urlPath.substr(7); // Remove "/media/"

    cout << "Requested filePath: " << filePath << "\n";

    // Build full filesystem path
    fs::path fullPath = fs::path("../media") / filePath;
    cout << "Full path: " << fullPath << "\n";

    // Get absolute/canonical paths for comparison
    fs::path mediaDir = fs::absolute(fs::path("../media"));
    fs::path requestedPath = fs::absolute(fullPath);

    cout << "Media dir absolute: " << mediaDir << "\n";
    cout << "Requested absolute: " << requestedPath << "\n";

    // Check if requested path is within media directory
    auto mediaDirStr = mediaDir.string();
    auto requestedStr = requestedPath.string();

    if (requestedStr.find(mediaDirStr) != 0) {
      cout << "Security check failed!" << "\n";
      cout << "Media dir: " << mediaDirStr << "\n";
      cout << "Requested: " << requestedStr << "\n";
      res->writeStatus("403 Forbidden")->end("Access denied");
      return;
    }

    // Check if file exists
    if (!fs::exists(fullPath)) {
      res->writeStatus("404 Not Found")->end("File not found");
      return;
    }

    // Open and serve file
    ifstream file(fullPath, ios::binary);
    if (!file.is_open()) {
      res->writeStatus("500 Internal Server Error")->end("Cannot open file");
      return;
    }

    // Read file into buffer
    vector<char> buffer((istreambuf_iterator<char>(file)),
                             istreambuf_iterator<char>());

    // Set content type
    res->writeHeader("Content-Type", "image/webp");
    res->writeHeader("Content-Length", to_string(buffer.size()));

    // Send file
    res->end(string_view(buffer.data(), buffer.size()));
  });

  // Upload route
  app.post("/upload", [&pubKey, &uuidGenerator](auto *res, auto *req) {
    if (rateLimit(res)) {
      return;
    };

    auto token = req->getHeader("x-twivo-backend");
    if (token.empty()) {
      res->writeStatus("400 Bad Request")->end("Missing JWT token");
      return;
    }

    auto decodedOpt = verifyUploadImageGetUserId(token, pubKey, *redisPtr);
    if (!decodedOpt.has_value()) {
        res->writeStatus("401 Unauthorized")->end("Invalid or expired token");
        return;
    }

    const auto& decoded = decodedOpt.value();

    // --- Check subject (sub) claim ---
    if (!decoded.has_subject()) {
        res->writeStatus("401 Unauthorized")->end("Missing user identifier (sub)");
        return;
    }
    auto userId = decoded.get_subject();

    // --- Safe extraction of "id" claim ---
    if (!decoded.has_payload_claim("id")) {
        res->writeStatus("401 Unauthorized")->end("Missing id claim");
        return;
    }
    string twiId;
    try {
        twiId = decoded.get_payload_claim("id").as_string();
    } catch (const std::bad_cast& e) {
        cerr << "Invalid type for id claim: " << e.what() << "\n";
        res->writeStatus("401 Unauthorized")->end("Invalid id claim type");
        return;
    }
    // ------------------------------------

    auto totalSize = make_shared<size_t>(0);
    auto buffer = make_shared<vector<unsigned char>>();
    auto fileType = make_shared<ImageType>(ImageType::UNKNOWN);

    // Capture userId and twiId by value
    res->onData([res, buffer, fileType, totalSize, userId, twiId, &uuidGenerator](string_view chunk, bool isLast) mutable {
        *totalSize += chunk.size();

        // Reject large uploads
        if (*totalSize > MAX_UPLOAD_SIZE) {
            res->end("File too large");
            return;
        }

        // Append chunk to buffer
        buffer->insert(buffer->end(), chunk.begin(), chunk.end());

        // Detect image type on first chunk
        if (buffer->size() >= magicbytes && *fileType == ImageType::UNKNOWN) {
            *fileType = getImageType(static_cast<const char *>(static_cast<const void *>(buffer->data())),
                                     buffer->size());
            if (*fileType == ImageType::UNKNOWN) {
                res->end("Invalid image format");
                return;
            }
        }

        if (isLast) {
            if (*fileType == ImageType::UNKNOWN) {
                res->end("Invalid or unsupported image");
                return;
            }

            // Create directories
            auto dir = fs::path("../media");
            fs::create_directories(dir);

            // Use userId as string, not pointer
            const string sub1 = userId.substr(0, 2);
            const string sub2 = userId.substr(2, 4);
            const string sub3 = userId.substr(4);

            // Build the full directory path
            auto fullDir = dir / sub1 / sub2 / sub3;

            // Create all nested directories
            error_code ec;
            fs::create_directories(fullDir, ec);
            if (ec) {
                cerr << "Failed to create directories: " << ec.message() << "\n";
                res->writeStatus("500 Internal Server Error")
                    ->end("Failed to create storage directory");
                return;
            }

            // Process image: resize + WebP conversion
            auto processed = ImageProcessor::loadResizeWebP(buffer->data(), buffer->size());

            if (!processed) {
                res->end("Failed to process image");
                return;
            }

            // Generate UUID using the library
            const auto uuid = uuidGenerator.getUUID().str();
            const string filename = (fullDir / (uuid + ".webp")).string();
            const string filepath = (fs::path(sub1) / sub2 / sub3 / (uuid + ".webp")).string();
            ofstream file(filename, ios::binary);
            if (!file.is_open()) {
                res->end("Failed to save file");
                return;
            }

            file.write(reinterpret_cast<const char *>(processed->webpData.data()),
                       processed->webpData.size());
            file.close();

            string orientation_str;
            switch (processed->orientation) {
                case ImageOrientation::HORIZONTAL:
                    orientation_str = "horizontal";
                    break;
                case ImageOrientation::VERTICAL:
                    orientation_str = "vertical";
                    break;
                case ImageOrientation::SQUARE:
                    orientation_str = "square";
                    break;
                default:
                    orientation_str = "unknown";
            }

            // Use twiId and userId directly (they are strings)
            vector<pair<string, string>> fields = {
                {"id", twiId},
                {"user_id", userId},
                {"path", filepath},
                {"orientation", orientation_str}
            };

            // Add to Redis stream
            redis.xadd("uploads:stream", "*", fields.begin(), fields.end());

            cout << "Upload event published for user: " << userId << "\n";
            res->end("Upload successful");
        }
    });

    res->onAborted([buffer, fileType, totalSize, userId]() {
        buffer->clear();
        *totalSize = 0;
        *fileType = ImageType::UNKNOWN;
        cout << "Upload aborted for user: " << userId << '\n';
    });
  });

  // Start server
  app.listen(PORT,
             [](auto *listen_socket) {
               if (listen_socket)
                 cout << "Server running at http://localhost:" << PORT << '\n';
             })
      .run();
}