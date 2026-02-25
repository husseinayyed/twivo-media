#pragma once
#include <string>
#include <memory>
#include <vector>

enum class ImageOrientation {
    VERTICAL,
    HORIZONTAL,
    SQUARE,
    INVALID
};

struct ProcessedImage {
    int width;
    int height;
    ImageOrientation orientation;
    std::vector<uint8_t> webpData; // Encoded WebP
};

class ImageProcessor {
public:
    static constexpr int MAX_DIMENSION = 2000;

    // Preset sizes
    static constexpr int SQUARE_SIZE = 600;
    static constexpr int HORIZONTAL_WIDTH = 600;
    static constexpr int HORIZONTAL_HEIGHT = 314;
    static constexpr int VERTICAL_WIDTH = 600;
    static constexpr int VERTICAL_HEIGHT = 750;

    // Load from memory, resize, convert to WebP quality 85
    static std::unique_ptr<ProcessedImage> loadResizeWebP(
        const unsigned char* data,
        size_t size
    );
};