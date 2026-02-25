#include "ImageType.hpp"
#include <cstddef>
#include <array>
#include <span>

namespace {
    constexpr unsigned char JPEG_SOI_0 = 0xFF;
    constexpr unsigned char JPEG_SOI_1 = 0xD8;
    constexpr unsigned char JPEG_APP0 = 0xFF;
    constexpr size_t JPEG_MIN_SIZE = 3;

    constexpr std::array<unsigned char, 8> PNG_SIGNATURE = {
        0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'
    };
    constexpr size_t PNG_MIN_SIZE = 8;

    constexpr std::array<unsigned char, 4> RIFF_SIGNATURE = {'R', 'I', 'F', 'F'};
    constexpr std::array<unsigned char, 4> WEBP_SIGNATURE = {'W', 'E', 'B', 'P'};
    constexpr size_t WEBP_MIN_SIZE = 12;
    constexpr size_t WEBP_WEBP_OFFSET = 8;

    template<size_t N>
    auto matchesSignature(std::span<const char> data, 
                          const std::array<unsigned char, N>& signature, 
                          const size_t offset = 0) -> bool {
        if (offset + N > data.size()) {
            return false;
        }
        
        auto slice = data.subspan(offset, N);
        for (size_t i = 0; i < N; ++i) {
            // NOLINTNEXTLINE(cppcoreguidelines-pro-bounds-constant-array-index)
            if (static_cast<unsigned char>(slice[i]) != signature[i]) {
                return false;
            }
        }
        return true;
    }
}

auto getImageType(const char* data, size_t len) -> ImageType
{
    if (!data || len == 0) {
        return ImageType::UNKNOWN;
    }

    const std::span<const char> dataSpan(data, len);

    // JPEG: FF D8 FF
    if (len >= JPEG_MIN_SIZE &&
        static_cast<unsigned char>(dataSpan[0]) == JPEG_SOI_0 &&
        static_cast<unsigned char>(dataSpan[1]) == JPEG_SOI_1 &&
        static_cast<unsigned char>(dataSpan[2]) == JPEG_APP0)
    {
        return ImageType::JPG;
    }

    // PNG: 89 50 4E 47 0D 0A 1A 0A
    if (matchesSignature(dataSpan, PNG_SIGNATURE)) {
        return ImageType::PNG;
    }

    // WebP: RIFF....WEBP
    if (len >= WEBP_MIN_SIZE &&
        matchesSignature(dataSpan, RIFF_SIGNATURE, 0) &&
        matchesSignature(dataSpan, WEBP_SIGNATURE, WEBP_WEBP_OFFSET))
    {
        return ImageType::WEBP;
    }

    return ImageType::UNKNOWN;
}