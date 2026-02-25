#pragma once
#include <cstddef>  // for size_t

enum class ImageType
{
    UNKNOWN,
    JPG,
    PNG,
    WEBP
};

ImageType getImageType(const char* data, size_t len);