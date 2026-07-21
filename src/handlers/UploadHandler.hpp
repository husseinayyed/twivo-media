#pragma once
#include <string>
#include <memory>
#include <string_view>
#include <vector>
#include <uwebsockets/App.h> // Essential for uWS types
#include "models/UploadContext.hpp"
#include "Core/Constants.hpp"

class SeaweedService;

typedef std::shared_ptr<UploadContext> UploadContextPtr;

class UploadHandler {
public:
    UploadHandler();
    ~UploadHandler();

    template<typename Res, typename Req>
    void handle(Res* res, Req* req, const std::string& pubKeyString) {
        // 1. Extract trusted identity headers set by the Nginx Gateway.
    // The Gateway guarantees these headers contain verified data.
    std::string userId = std::string(req->getHeader("x-twivo-user-id"));
    std::string tweetId = std::string(req->getHeader("x-twivo-tweet-id"));

    // 2. Defensive check: Ensure the request actually passed through our Gateway.
    // If these are empty, an unauthorized user tried to access the backend directly.
    if (userId.empty() || tweetId.empty()) {
        res->writeStatus("403 Forbidden")->end("Security Violation: Unauthorized Access");
        return;
    }

    // 3. Initialize the context with the verified identity.
    auto ctx = std::make_shared<UploadContext>();
    ctx->userId = userId;
    ctx->tweetId = tweetId;

    // 4. Handle incoming data chunks.
    res->onData([this, res, ctx](std::string_view chunk, bool isLast) {
        if (ctx->isCompleted) return;

        // Check if the file size exceeds the allowed limit.
        ctx->totalSize += chunk.size();
        if (ctx->totalSize > constants::MAX_UPLOAD_SIZE) {
            ctx->isCompleted = true;
            res->writeStatus("413 Payload Too Large")->end("File too large");
            return;
        }

        // Buffer the data chunk.
        ctx->buffer.insert(ctx->buffer.end(), chunk.begin(), chunk.end());

        // Attempt to detect the image type once we have enough data.
        if (ctx->fileType == ImageType::UNKNOWN && ctx->buffer.size() >= constants::MIN_BUFFER_FOR_TYPE_DETECTION) {
            ctx->fileType = getImageType(reinterpret_cast<const char*>(ctx->buffer.data()), ctx->buffer.size());
        }

        // Finalize the request when the last chunk arrives.
        if (isLast) {
            ctx->isCompleted = true;
            // processImage now uses the trusted userId and tweetId stored in the context.
            if(processImage(ctx, res)){
                res->writeStatus("201 Created")->end("Upload successful");
                
            }
        }
    });

    res->onAborted([ctx]() {
        ctx->isCompleted = true;
        std::cerr << "⚠️ Upload aborted by client" << std::endl;
    });
    }

private:
    bool processImage(UploadContextPtr ctx, uWS::HttpResponse<false>* res);
    bool validateDimensions(UploadContextPtr ctx);
    
    std::unique_ptr<SeaweedService> seaweedService_;
};