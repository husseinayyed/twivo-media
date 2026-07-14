#pragma once
#include <string>
#include <memory>
#include <string_view>
#include <vector>
#include <uwebsockets/App.h> // Essential for uWS types
#include "models/UploadContext.hpp"
#include "Core/Constants.hpp"

class TokenValidator;
class SeaweedService;

typedef std::shared_ptr<UploadContext> UploadContextPtr;

class UploadHandler {
public:
    UploadHandler();
    ~UploadHandler();

    template<typename Res, typename Req>
    void handle(Res* res, Req* req, const std::string& pubKeyString) {
        auto token = req->getHeader("x-twivo-backend");
        if (token.empty()) {
            res->writeStatus("400 Bad Request")->end("Missing JWT token");
            return;
        }

        auto ctx = std::make_shared<UploadContext>();
        if (!authenticate(std::string(token), pubKeyString, ctx)) {
            res->writeStatus("401 Unauthorized")->end("Invalid or expired token");
            return;
        }

        res->onData([this, res, ctx](std::string_view chunk, bool isLast) {
            if (ctx->isCompleted) return;

            ctx->totalSize += chunk.size();
            if (ctx->totalSize > constants::MAX_UPLOAD_SIZE) {
                ctx->isCompleted = true;
                res->writeStatus("413 Payload Too Large")->end("File too large");
                return;
            }

            ctx->buffer.insert(ctx->buffer.end(), chunk.begin(), chunk.end());

            if (ctx->fileType == ImageType::UNKNOWN && ctx->buffer.size() >= constants::MIN_BUFFER_FOR_TYPE_DETECTION) {
                ctx->fileType = getImageType(reinterpret_cast<const char*>(ctx->buffer.data()), ctx->buffer.size());
            }

            if (isLast) {
                ctx->isCompleted = true;
                processImage(ctx, res);
            }
        });
    }

private:
    bool processImage(UploadContextPtr ctx, uWS::HttpResponse<false>* res);
    bool authenticate(const std::string& token, const std::string& key, UploadContextPtr ctx);
    bool validateDimensions(UploadContextPtr ctx);
    
    std::unique_ptr<TokenValidator> tokenValidator_;
    std::unique_ptr<SeaweedService> seaweedService_;
};