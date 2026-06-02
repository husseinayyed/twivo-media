// src/models/UploadContext.hpp
#pragma once

#include "ImageType.hpp"
#include <vector>
#include <memory>
#include <atomic>

/**
 * Upload state for a single request
 * Shared between the main handler and async callbacks
 */
struct UploadContext {
    // Size tracking
    size_t totalSize = 0;
    
    // Image data
    std::vector<uint8_t> buffer;
    
    // Image metadata
    ImageType fileType = ImageType::UNKNOWN;
    bool dimensionsChecked = false;
    
    // State management
    bool isCompleted = false;
    
    // User info
    std::string userId;
    std::string tweetId;
    
    // Reset context for reuse (if needed)
    void reset() {
        totalSize = 0;
        buffer.clear();
        fileType = ImageType::UNKNOWN;
        dimensionsChecked = false;
        isCompleted = false;
        userId.clear();
        tweetId.clear();
    }
    
    // Check if we have enough data to detect image type
    bool canDetectType() const {
        return buffer.size() >= 12;
    }
    
    // Check if upload exceeds size limit
    bool exceedsSizeLimit(size_t maxSize) const {
        return totalSize > maxSize;
    }
};

// Alias for shared pointer (convenience)
using UploadContextPtr = std::shared_ptr<UploadContext>;