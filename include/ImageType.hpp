#ifndef IMAGE_TYPE_HPP
#define IMAGE_TYPE_HPP

#include <cstddef>
#include <cstdint>
#include <span>
#include <string>

enum class ImageType {
    UNKNOWN,
    PNG,
    JPG,
    WEBP,
    TXT
};

struct ImageDimensions {
    uint32_t width = 0;
    uint32_t height = 0;
    bool valid = false;
};

auto getImageType(const char* data, size_t len) -> ImageType;
auto getImageDimensions(std::span<const char> data, ImageType type) -> ImageDimensions;
auto getExtensionString(ImageType type) -> std::string;

#endif // IMAGE_TYPE_HPP