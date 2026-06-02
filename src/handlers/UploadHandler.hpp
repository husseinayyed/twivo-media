#pragma once
#include <memory>
#include <string>
#include <vector>
#include "ImageType.hpp"
#include "TokenVerifier.hpp"
#include "Services/RedisService.hpp"
#include "Services/SeaweedService.hpp"
#include "Utils/CryptoUtils.hpp"
#include "Utils/ImageUtils.hpp"
#include "Core/Constants.hpp"
extern sw::redis::Redis* redisPtr;
class UploadHandler {
public:
    template<typename Res, typename Req>
    void handle(Res* res, Req* req, const std::string& pubKeyString) {
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

        if (!decoded.has_subject()) {
            res->writeStatus("401 Unauthorized")->end("Missing user identifier");
            return;
        }
        std::string userId = decoded.get_subject();

        if (!decoded.has_payload_claim("id")) {
            res->writeStatus("401 Unauthorized")->end("Missing id claim");
            return;
        }
        
        std::string twiId;
        try {
            twiId = decoded.get_payload_claim("id").as_string();
        } catch (const std::bad_cast& e) {
            res->writeStatus("401 Unauthorized")->end("Invalid id claim type");
            return;
        }

        // Shared state
        auto totalSize = std::make_shared<size_t>(0);
        auto buffer = std::make_shared<std::vector<uint8_t>>();
        auto fileType = std::make_shared<ImageType>(ImageType::UNKNOWN);
        auto dimensionsChecked = std::make_shared<bool>(false);
        auto isCompleted = std::make_shared<bool>(false);

        res->onData([res, buffer, fileType, totalSize, userId, twiId, dimensionsChecked, isCompleted](std::string_view chunk, bool isLast) mutable {
            if (*isCompleted) return;

            *totalSize += chunk.size();

            if (*totalSize > constants::MAX_UPLOAD_SIZE) {
                *isCompleted = true;
                res->writeStatus("413 Payload Too Large")->end("File too large");
                return;
            }

            buffer->insert(buffer->end(), chunk.begin(), chunk.end());

            // Detect image type
            if (buffer->size() >= constants::MIN_BUFFER_FOR_TYPE_DETECTION && *fileType == ImageType::UNKNOWN) {
                *fileType = getImageType(reinterpret_cast<const char*>(buffer->data()), buffer->size());
                if (*fileType == ImageType::UNKNOWN) {
                    *isCompleted = true;
                    res->writeStatus("400 Bad Request")->end("Invalid image format");
                    return;
                }
            }

            // Validate dimensions
            if (!*dimensionsChecked && *fileType != ImageType::UNKNOWN) {
                bool ready = false;
                switch (*fileType) {
                    case ImageType::PNG: ready = (buffer->size() >= 24); break;
                    case ImageType::WEBP: ready = (buffer->size() >= 30); break;
                    case ImageType::JPG: ready = (buffer->size() > 65536); break;
                    default: break;
                }
                if (ready) {
                    ImageDimensions dim = getImageDimensions(
                        std::span<const char>(reinterpret_cast<const char*>(buffer->data()), buffer->size()), 
                        *fileType
                    );
                    if (dim.valid) {
                        if (dim.width > constants::MAX_IMAGE_DIMENSION || dim.height > constants::MAX_IMAGE_DIMENSION) {
                            *isCompleted = true;
                            res->writeStatus("422 Unprocessable Entity")->end("Image too large");
                            return;
                        }
                        *dimensionsChecked = true;
                    }
                }
            }

            if (isLast) {
                *isCompleted = true;
                if (*fileType == ImageType::UNKNOWN || !*dimensionsChecked) {
                    res->writeStatus("400 Bad Request")->end("Invalid image");
                    return;
                }

                std::string hash = crypto::sha256(*buffer);
                std::string id = crypto::generateNanoId();
                std::string ext = image::getExtensionString(*fileType);
                std::string filepath = "/i/" + hash + ext;

                SeaweedService seaweed;
                if (!seaweed.upload(*buffer, filepath, ext.substr(1))) {
                    res->writeStatus("500 Internal Server Error")->end("Storage failed");
                    return;
                }

                ImageDimensions finalDim = getImageDimensions(
                    std::span<const char>(reinterpret_cast<const char*>(buffer->data()), buffer->size()), 
                    *fileType
                );
                std::string orientation = image::getOrientationString(finalDim);

                std::vector<std::pair<std::string, std::string>> fields = {
                    {"id", twiId}, {"user_id", userId}, {"path", id},
                    {"sha256", hash}, {"orientation", orientation}
                };
                RedisService::getInstance().addToStream(constants::REDIS_STREAM_NAME, fields);
                res->writeStatus("201 Created")->end("Upload successful - Path: " + filepath);
            }
        });

        res->onAborted([buffer, fileType, totalSize, userId, isCompleted]() {
            *isCompleted = true;
            buffer->clear();
            *totalSize = 0;
            *fileType = ImageType::UNKNOWN;
        });
    }
};