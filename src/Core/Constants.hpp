// src/core/Constants.hpp
#pragma once

#include <cstddef>

namespace constants {

    // Server configuration
    inline constexpr short int PORT = 8020;
    
    // Upload limits
    inline constexpr size_t MAX_UPLOAD_SIZE = 10 * 1024 * 1024;  // 10 MB
    inline constexpr uint32_t MAX_IMAGE_DIMENSION = 2000;       // pixels
    
    // Timeouts
    inline constexpr int REQUEST_TIMEOUT_MS = 30000;    // 30 seconds
    inline constexpr int CURL_TIMEOUT_SEC = 30;         // 30 seconds
    inline constexpr int MAX_RETRIES = 3;               // for SeaweedFS
    
    // Image detection
    inline constexpr size_t MIN_BUFFER_FOR_TYPE_DETECTION = 12;   // bytes needed to detect type
    inline constexpr size_t JPEG_MAX_SCAN_BUFFER = 65536;         // max bytes to scan for JPEG markers
    
    // External services
    inline constexpr const char* SEAWEEDFS_FILER_URL = "http://weed-filer:8888";
    inline constexpr const char* DEFAULT_PUBLIC_KEY_PATH = "/app/keys/public.pem";
    
    // Redis
    inline constexpr const char* REDIS_STREAM_NAME = "uploads:stream";
    
} // namespace constants