#include "ImageProcessor.hpp"
#include "ImageType.hpp"
#include "TokenVerifier.hpp"
#include "uuid/v4/uuid.h"
#include <App.h>
#include <cstddef>
#include <cstdlib>
#include <filesystem>
#include <format>
#include <fstream>
#include <iostream>
#include <iterator>
#include <memory>
#include <string>
#include <sw/redis++/redis++.h>
using namespace std;
using namespace sw::redis;
namespace fs = std::filesystem;

constexpr short int PORT = 8080;
constexpr short int magicbytes = 12;
constexpr size_t MAX_UPLOAD_SIZE =
    static_cast<size_t>(10 * 1024 * 1024); // 10 MB
static Redis redis("tcp://127.0.0.1:6379");

auto readPublicKey() -> string {
  ifstream f("../twivo-backend.pem");
  if (!f.is_open()) {
    cerr << "ERROR: Cannot open twivo-backend.pem" << '\n';
    exit(1);
  }
  string key((std::istreambuf_iterator<char>(f)),
             std::istreambuf_iterator<char>());
  if (key.empty()) {
    cerr << "ERROR: Empty twivo-backend.pem" << '\n';
    exit(1);
  }
  f.close();
  return key;
}
auto rateLimit(auto res) -> bool {
  const std::string ip = std::string(res->getRemoteAddressAsText());
  const int limit = 5; // Maximum requests per minute

  std::string script = R"(
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
                              {std::to_string(limit)});            // ARGV
    if (!result) {
      res->writeStatus("429 Too Many Requests")->end("Too many requests");
      return true;
    }
  } catch (const sw::redis::ReplyError &e) {
    cerr << "Redis error: " << e.what() << endl;
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

  // Simple GET route
  app.get("/*", [](auto *res, auto * /*req*/) {
    if (rateLimit(res)) {
      return;
    };
    res->end("hello world!");
  });
  // Upload route
  app.post("/upload", [&pubKey](auto *res, auto *req) {
    if (rateLimit(res)) {
      return;
    };
    auto token = req->getHeader("x-twivo-backend"); // try lowercase
    if (token.empty()) {
      res->writeStatus("400 Bad Request")->end("Missing JWT token");
      return;
    }
    auto userId =
        make_shared<string>(verifyUploadImageGetUserId(token, pubKey));

    if (userId->empty()) {
      res->writeStatus("401 Unauthorized")->end("Invalid or expired token");
      return;
    }
    auto totalSize = make_shared<size_t>(0);
    auto buffer = make_shared<string>();
    auto fileType = make_shared<ImageType>(ImageType::UNKNOWN);

    res->onData([res, buffer, fileType, totalSize,
                 userId](string_view chunk, bool isLast) mutable {
      *totalSize += chunk.size();

      // Reject large uploads
      if (*totalSize > MAX_UPLOAD_SIZE) {
        res->end("File too large");
        return;
      }

      buffer->append(chunk);

      // Detect image type on first chunk
      if (buffer->size() >= magicbytes && *fileType == ImageType::UNKNOWN) {
        *fileType = getImageType(buffer->data(), buffer->size());
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
        auto dir = fs::path(("../media"));
        fs::create_directories(dir);
        // Create nested directories from userId
        const string sub1 = userId->substr(0, 2);
        const string sub2 = userId->substr(2, 4);
        const string sub3 = userId->substr(4);

        // Build the full directory path
        auto fullDir = dir / sub1 / sub2 / sub3;

        // Create all nested directories
        std::error_code ec;
        fs::create_directories(fullDir, ec);
        if (ec) {
          cerr << "Failed to create directories: " << ec.message() << endl;
          res->writeStatus("500 Internal Server Error")
              ->end("Failed to create storage directory");
          return;
        }

        // Generate UUID and filename
        // Process image: resize + WebP conversion
        auto processed = ImageProcessor::loadResizeWebP(
            static_cast<const unsigned char *>(
                static_cast<const void *>(buffer->data())),
            buffer->size());

        if (!processed) {
          res->end("Failed to process image");
          return;
        }
        const auto uuid = uuid::v4::UUID::New().String();
        const string filename = (fullDir / (uuid + ".webp")).string();
        ofstream file(filename, ios::binary);
        if (!file.is_open()) {
          res->end("Failed to save file");
          return;
        }

        file.write(static_cast<char *>(
                       static_cast<void *>(processed->webpData.data())),
                   static_cast<std::streamsize>(processed->webpData.size()));
        file.close();

        res->end("Upload successful");
      }
    });

    res->onAborted([buffer, fileType, totalSize, userId]() {
      buffer->clear();
      *totalSize = 0;
      *fileType = ImageType::UNKNOWN;
      cout << "Upload aborted for user: " << *userId << '\n';
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