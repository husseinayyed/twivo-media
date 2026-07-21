#include "UploadHandler.hpp"
#include "Services/SeaweedService.hpp"
#include "Services/RedisService.hpp"
#include "Utils/ImageUtils.hpp"
#include "Utils/CryptoUtils.hpp"
#include "Core/Constants.hpp"
#include <iostream>
#include <span>

UploadHandler::UploadHandler() : seaweedService_(std::make_unique<SeaweedService>()) {}

UploadHandler::~UploadHandler() = default;



bool UploadHandler::validateDimensions(UploadContextPtr ctx) {
    auto dim = getImageDimensions(
        std::span<const char>(reinterpret_cast<const char*>(ctx->buffer.data()), ctx->buffer.size()), 
        ctx->fileType
    );
    
    if (!dim.valid) return true;
    
    if (dim.width > constants::MAX_IMAGE_DIMENSION || dim.height > constants::MAX_IMAGE_DIMENSION) {
        return false;
    }
    return true;
}

bool UploadHandler::processImage(UploadContextPtr ctx, uWS::HttpResponse<false>* res) {
    std::string hash = crypto::sha256(ctx->buffer);
    std::string id = crypto::generateNanoId();
    std::string ext = image::getExtensionString(ctx->fileType);
    std::string filepath = "/i/" + hash + ext;
    
    if (!seaweedService_->upload(ctx->buffer, filepath, ext.substr(1))) {
        res->writeStatus("500 Internal Server Error")->end("Storage engine error");
        return false;
    }
    
    auto dim = getImageDimensions(
        std::span<const char>(reinterpret_cast<const char*>(ctx->buffer.data()), ctx->buffer.size()), 
        ctx->fileType
    );
    
    std::string orientation = image::getOrientationString(dim);
    int width = (orientation == "vertical") ? 300 : 600;
    int height = (orientation == "vertical") ? 714 : 600;

    std::string seaweed_url = "http://weed-filer:8888/i/" + hash + ext;
    std::string imgproxy_path = "rs:fill:" + std::to_string(width) + ":" + std::to_string(height) +
                                "/format:webp/quality:85/dpr:2/plain/" + seaweed_url;
                                
    auto& redis = RedisService::getInstance();
    redis.storeNanoId(id, imgproxy_path, 3600);

    std::vector<std::pair<std::string, std::string>> fields = {
        {"id", ctx->tweetId}, {"user_id", ctx->userId}, {"path", id},
        {"sha256", hash}, {"orientation", orientation}
    };

    redis.addToStream(constants::REDIS_STREAM_NAME, fields);
    
    
    return true;
}