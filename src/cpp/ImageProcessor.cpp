#include "ImageProcessor.hpp"

#define STB_IMAGE_IMPLEMENTATION
#include "stb_image.h"

#define STB_IMAGE_RESIZE_IMPLEMENTATION
#include "stb_image_resize2.h"

#define STB_IMAGE_WRITE_IMPLEMENTATION
#include "stb_image_write.h"

#include <webp/encode.h>

std::unique_ptr<ProcessedImage> ImageProcessor::loadResizeWebP(
    const unsigned char* data,
    size_t size
) {
    int w, h, comp;

    // Read image header only
    if (!stbi_info_from_memory(data, size, &w, &h, &comp)) return nullptr;
    if (w > MAX_DIMENSION || h > MAX_DIMENSION) return nullptr;

    // Decode full image as RGBA
    unsigned char* img = stbi_load_from_memory(data, size, &w, &h, &comp, 4);
    if (!img) return nullptr;

    // Determine orientation and target size
    ImageOrientation orient;
    int target_w, target_h;

    if (w > h) {
        orient = ImageOrientation::HORIZONTAL;
        target_w = HORIZONTAL_WIDTH;
        target_h = HORIZONTAL_HEIGHT;
    } else if (h > w) {
        orient = ImageOrientation::VERTICAL;
        target_w = VERTICAL_WIDTH;
        target_h = VERTICAL_HEIGHT;
    } else {
        orient = ImageOrientation::SQUARE;
        target_w = SQUARE_SIZE;
        target_h = SQUARE_SIZE;
    }

    // Resize to preset
    std::unique_ptr<unsigned char[]> resized = std::make_unique<unsigned char[]>(target_w * target_h * 4);
    stbir_resize_uint8_linear(
        img, w, h, 0,
        resized.get(), target_w, target_h, 0,
        STBIR_RGBA
    );
    stbi_image_free(img);
    float quality = 100.0f;
    uint8_t* webp_buf = nullptr;
    size_t webp_size = WebPEncodeRGBA(resized.get(), target_w, target_h, target_w*4, quality, &webp_buf);
    if (!webp_size) return nullptr;

    // Copy WebP into vector for safe ownership
    std::vector<uint8_t> webpData(webp_buf, webp_buf + webp_size);
    WebPFree(webp_buf);

    // Build final ProcessedImage
    auto processed = std::make_unique<ProcessedImage>();
    processed->width = target_w;
    processed->height = target_h;
    processed->orientation = orient;
    processed->webpData = std::move(webpData);

    return processed;
}