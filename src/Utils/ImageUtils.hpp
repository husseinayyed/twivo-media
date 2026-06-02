// src/utils/ImageUtils.hpp
#pragma once

#include "ImageType.hpp"
#include <string>
#include <cstdint>
namespace image {

    /**
     * Get file extension for image type
     * @param type Image type (PNG, JPG, WEBP)
     * @return File extension with dot (e.g., ".png")
     */
    std::string getExtensionString(ImageType type);
    
    /**
     * Get orientation string based on dimensions
     * @param dim Image dimensions
     * @return "horizontal", "vertical", "square", or "unknown"
     */
    std::string getOrientationString(const ImageDimensions& dim);
    
    /**
     * Check if we have enough data to validate dimensions
     * @param type Image type
     * @param buffer_size Current buffer size
     * @return true if ready to validate
     */
    bool isReadyForDimensionValidation(ImageType type, size_t buffer_size);
    
} // namespace image