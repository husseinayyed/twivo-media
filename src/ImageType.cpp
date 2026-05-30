#include "ImageType.hpp"
#include <cstddef>
#include <array>
#include <span>
#include <string>
#include <string_view>
#include <cstdint>

namespace {
    constexpr unsigned char JPEG_SOI_0 = 0xFF;
    constexpr unsigned char JPEG_SOI_1 = 0xD8;
    constexpr size_t JPEG_MIN_SIZE = 2; // Fixed: Redefined boundary requirement to match core specs

    constexpr std::array<unsigned char, 8> PNG_SIGNATURE = {
        0x89, 'P', 'N', 'G', '\r', '\n', 0x1A, '\n'
    };

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
            if (static_cast<unsigned char>(slice[i]) != signature[i]) {
                return false;
            }
        }
        return true;
    }

    auto readUint16BE(std::span<const char> data, size_t offset) -> uint16_t {
        return (static_cast<uint16_t>(static_cast<unsigned char>(data[offset])) << 8) |
               static_cast<uint16_t>(static_cast<unsigned char>(data[offset + 1]));
    }

    auto readUint32BE(std::span<const char> data, size_t offset) -> uint32_t {
        return (static_cast<uint32_t>(static_cast<unsigned char>(data[offset])) << 24) |
               (static_cast<uint32_t>(static_cast<unsigned char>(data[offset + 1])) << 16) |
               (static_cast<uint32_t>(static_cast<unsigned char>(data[offset + 2])) << 8) |
               (static_cast<uint32_t>(static_cast<unsigned char>(data[offset + 3])));
    }
}



auto getImageDimensions(std::span<const char> data, ImageType type) -> ImageDimensions {
    ImageDimensions dim;
    const size_t len = data.size();

    switch (type) {
        case ImageType::PNG: {
            if (len >= 24) {
                dim.width = readUint32BE(data, 16);
                dim.height = readUint32BE(data, 20);
                dim.valid = true;
            }
            break;
        }
        case ImageType::JPG: {
            size_t i = 2; // Skip SOI marker (FF D8)
            while (i + 8 < len) {
                if (static_cast<unsigned char>(data[i]) != 0xFF) {
                    break; 
                }
                
                unsigned char marker = static_cast<unsigned char>(data[i + 1]);
                // SOF0 (Baseline) or SOF2 (Progressive) markers containing dimension fields
                if (marker == 0xC0 || marker == 0xC2) {
                    dim.height = readUint16BE(data, i + 5);
                    dim.width = readUint16BE(data, i + 7);
                    dim.valid = true;
                    break;
                } else {
                    size_t markerLength = readUint16BE(data, i + 2);
                    if (i + 2 + markerLength >= len || markerLength < 2) {
                        break; 
                    }
                    i += 2 + markerLength;
                }
            }
            break;
        }
        case ImageType::WEBP: {
            if (len < 30) break;
            
            std::string_view formatType(&data[12], 4);
            
            if (formatType == "VP8 " && len >= 30) {
                dim.width = (static_cast<unsigned char>(data[26]) | (static_cast<unsigned char>(data[27]) << 8)) & 0x3FFF;
                dim.height = (static_cast<unsigned char>(data[28]) | (static_cast<unsigned char>(data[29]) << 8)) & 0x3FFF;
                dim.valid = true;
            } 
            else if (formatType == "VP8L" && len >= 25) {
                uint32_t b0 = static_cast<unsigned char>(data[21]);
                uint32_t b1 = static_cast<unsigned char>(data[22]);
                uint32_t b2 = static_cast<unsigned char>(data[23]);
                uint32_t b3 = static_cast<unsigned char>(data[24]);
                
                dim.width = 1 + (b0 | ((b1 & 0x3F) << 8));
                dim.height = 1 + (((b1 >> 6) | (b2 << 2) | ((b3 & 0x0F) << 10)));
                dim.valid = true;
            } 
            else if (formatType == "VP8X" && len >= 30) {
                dim.width = 1 + (static_cast<unsigned char>(data[24]) | 
                                (static_cast<unsigned char>(data[25]) << 8) | 
                                (static_cast<unsigned char>(data[26]) << 16));
                dim.height = 1 + (static_cast<unsigned char>(data[27]) | 
                                 (static_cast<unsigned char>(data[28]) << 8) | 
                                 (static_cast<unsigned char>(data[29]) << 16));
                dim.valid = true;
            }
            break;
        }
        default:
            break;
    }
    return dim;
}

auto getImageType(const char* data, size_t len) -> ImageType {
    if (!data || len == 0) {
        return ImageType::UNKNOWN;
    }

    const std::span<const char> dataSpan(data, len);

    // Fixed: Checking for raw magic bytes (FF D8) exclusively to capture all JPEG formats accurately
    if (len >= JPEG_MIN_SIZE &&
        static_cast<unsigned char>(dataSpan[0]) == JPEG_SOI_0 &&
        static_cast<unsigned char>(dataSpan[1]) == JPEG_SOI_1)
    {
        return ImageType::JPG;
    }

    if (matchesSignature(dataSpan, PNG_SIGNATURE)) {
        return ImageType::PNG;
    }

    if (len >= WEBP_MIN_SIZE &&
        matchesSignature(dataSpan, RIFF_SIGNATURE, 0) &&
        matchesSignature(dataSpan, WEBP_SIGNATURE, WEBP_WEBP_OFFSET))
    {
        return ImageType::WEBP;
    }

    return ImageType::UNKNOWN;
}
