// src/utils/ImageUtils.cpp
#include "ImageUtils.hpp"
#include "Core/Constants.hpp"

namespace image {

    std::string getExtensionString(ImageType type) {
        switch (type) {
            case ImageType::PNG:  return ".png";
            case ImageType::JPG:  return ".jpg";
            case ImageType::WEBP: return ".webp";
            default:              return "";
        }
    }
    
    std::string getOrientationString(const ImageDimensions& dim) {
        if (!dim.valid) {
            return "unknown";
        }
        
        if (dim.width > dim.height) {
            return "horizontal";
        } else if (dim.height > dim.width) {
            return "vertical";
        } else {
            return "square";
        }
    }
    
    bool isReadyForDimensionValidation(ImageType type, size_t buffer_size) {
        switch (type) {
            case ImageType::PNG:
                // PNG stores dimensions in IHDR chunk at offset 16
                return buffer_size >= 24;
                
            case ImageType::WEBP:
                // WebP stores dimensions in header at offset 24-29
                return buffer_size >= 30;
                
            case ImageType::JPG:
                // JPEG requires scanning for SOF markers
                // If buffer is large enough, we can scan
                return buffer_size > constants::JPEG_MAX_SCAN_BUFFER;
                
            default:
                return false;
        }
    }
    
} // namespace image