// src/handlers/UploadHandler.cpp
#include "UploadHandler.hpp"
#include "auth/TokenValidator.hpp"
#include "Services/SeaweedService.hpp"
#include "Services/RedisService.hpp"
#include "Utils/ImageUtils.hpp"
#include "Utils/CryptoUtils.hpp"
#include "Core/Constants.hpp"
#include <iostream>
#include <span>

UploadHandler::UploadHandler() 
    : tokenValidator_(std::make_unique<TokenValidator>())
    , seaweedService_(std::make_unique<SeaweedService>()){
}

UploadHandler::~UploadHandler() = default;
template<typename Res, typename Req>
void UploadHandler::handle(Res* res, Req* req, const std::string& publicKey) {
    // Set timeout for the request
    res->setTimeout(constants::REQUEST_TIMEOUT_MS);
    
    // Get JWT token from header
    auto token = req->getHeader("x-twivo-backend");
    if (token.empty()) {
        res->writeStatus("400 Bad Request")
           ->writeHeader("Content-Type", "text/plain")
           ->end("Missing JWT token");
        return;
    }
    
    // Create upload context
    auto ctx = std::make_shared<UploadContext>();
    
    // Authenticate and extract user info
    if (!authenticate(token, publicKey, ctx)) {
        res->writeStatus("401 Unauthorized")
           ->writeHeader("Content-Type", "text/plain")
           ->end("Invalid or expired token");
        return;
    }
    
    std::cout << "📥 Processing upload for user: " << ctx->userId 
              << ", tweet: " << ctx->tweetId << std::endl;
    
    // Handle incoming data chunks
    res->onData([this, res, ctx](std::string_view chunk, bool isLast) {
        if (ctx->isCompleted) return;
        
        // Update size
        ctx->totalSize += chunk.size();
        
        // Check size limit
        if (ctx->exceedsSizeLimit(constants::MAX_UPLOAD_SIZE)) {
            ctx->isCompleted = true;
            res->writeStatus("413 Payload Too Large")
               ->writeHeader("Content-Type", "text/plain")
               ->end("File too large (max 10MB)");
            return;
        }
        
        // Append data to buffer
        ctx->buffer.insert(ctx->buffer.end(), chunk.begin(), chunk.end());
        
        // Detect image type (if not already detected)
        if (ctx->fileType == ImageType::UNKNOWN && ctx->canDetectType()) {
            ctx->fileType = getImageType(
                reinterpret_cast<const char*>(ctx->buffer.data()), 
                ctx->buffer.size()
            );
            
            if (ctx->fileType == ImageType::UNKNOWN) {
                ctx->isCompleted = true;
                res->writeStatus("400 Bad Request")
                   ->writeHeader("Content-Type", "text/plain")
                   ->end("Unsupported or invalid image format");
                return;
            }
            
            std::cout << "📷 Detected image type: " 
                      << image::getExtensionString(ctx->fileType) << std::endl;
        }
        
        // Validate dimensions (if enough data is available)
        if (!ctx->dimensionsChecked && ctx->fileType != ImageType::UNKNOWN) {
            if (image::isReadyForDimensionValidation(ctx->fileType, ctx->buffer.size())) {
                if (!validateDimensions(ctx)) {
                    ctx->isCompleted = true;
                    return; // Response already sent in validateDimensions
                }
                ctx->dimensionsChecked = true;
            }
        }
        
        // Process when upload is complete
        if (isLast) {
            ctx->isCompleted = true;
            
            if (ctx->fileType == ImageType::UNKNOWN || !ctx->dimensionsChecked) {
                res->writeStatus("400 Bad Request")
                   ->writeHeader("Content-Type", "text/plain")
                   ->end("Incomplete or invalid image data");
                return;
            }
            
            processImage(ctx, res);
        }
    });
    
    // Handle client disconnect
    res->onAborted([ctx]() {
        ctx->isCompleted = true;
        ctx->buffer.clear();
        std::cout << "⚠️ Upload aborted by client" << std::endl;
    });
}

bool UploadHandler::authenticate(const std::string& token, 
                                  const std::string& publicKey,
                                  UploadContextPtr ctx) {
    return tokenValidator_->verify(token, publicKey, ctx->userId, ctx->tweetId);
}

bool UploadHandler::validateDimensions(UploadContextPtr ctx) {
    auto dim = getImageDimensions(
        std::span<const char>(
            reinterpret_cast<const char*>(ctx->buffer.data()), 
            ctx->buffer.size()
        ), 
        ctx->fileType
    );
    
    if (!dim.valid) {
        // Can't validate yet, wait for more data
        return true;
    }
    
    if (dim.width > constants::MAX_IMAGE_DIMENSION || 
        dim.height > constants::MAX_IMAGE_DIMENSION) {
        std::cerr << "❌ Image too large: " << dim.width << "x" << dim.height << std::endl;
        return false;
    }
    
    std::cout << "✅ Dimensions valid: " << dim.width << "x" << dim.height << std::endl;
    return true;
}

bool UploadHandler::processImage(UploadContextPtr ctx, auto* res) {
    // Generate content-addressed path
    std::string hash = crypto::sha256(ctx->buffer);
    std::string id = crypto::generateNanoId();
    std::string ext = image::getExtensionString(ctx->fileType);
    std::string filepath = "/i/" + hash + ext;
    
    std::cout << "📁 Generated path: " << filepath << std::endl;
    std::cout << "🔑 SHA256: " << hash << std::endl;
    
    // Upload to SeaweedFS
    if (!seaweedService_->upload(ctx->buffer, filepath, ext.substr(1))) { // Remove dot from extension
        res->writeStatus("500 Internal Server Error")
           ->writeHeader("Content-Type", "text/plain")
           ->end("Storage engine error");
        return false;
    }
    
    // Get final dimensions for orientation
    auto dim = getImageDimensions(
        std::span<const char>(
            reinterpret_cast<const char*>(ctx->buffer.data()), 
            ctx->buffer.size()
        ), 
        ctx->fileType
    );
    std::string orientation = image::getOrientationString(dim);
    
    // Save metadata to Redis
    auto& redis = RedisService::getInstance();
    std::vector<std::pair<std::string, std::string>> fields = {
        {"id", ctx->tweetId},
        {"user_id", ctx->userId},
        {"path", id},
        {"sha256", hash},
        {"orientation", orientation}
    };
    
    if (!redis.addToStream(constants::REDIS_STREAM_NAME, fields)) {
        std::cerr << "⚠️ Failed to save metadata to Redis" << std::endl;
        // Continue anyway - file is already uploaded
    }
    
    // Send success response
    std::string response = "Upload successful - Path: " + filepath;
    res->writeStatus("201 Created")
       ->writeHeader("Content-Type", "text/plain")
       ->end(response);
    
    std::cout << "✅ Upload complete: " << filepath << std::endl;
    return true;
}