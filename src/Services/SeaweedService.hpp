// src/services/SeaweedService.hpp
#pragma once

#include <vector>
#include <string>

class SeaweedService {
public:
    /**
     * Upload file to SeaweedFS
     * @param data File binary data
     * @param filepath Path in SeaweedFS (e.g., "/i/hash.jpg")
     * @param extension File extension (without dot, e.g., "jpg")
     * @return true if upload successful
     */
    bool upload(const std::vector<uint8_t>& data, 
                const std::string& filepath, 
                const std::string& extension);
                
private:
    bool uploadWithRetry(const std::vector<uint8_t>& data,
                         const std::string& url,
                         const std::string& filename,
                         const std::string& mimeType,
                         int attempt);
};