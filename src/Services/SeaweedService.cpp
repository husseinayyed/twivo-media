// src/services/SeaweedService.cpp
#include "SeaweedService.hpp"
#include "Core/Constants.hpp"
#include <curl/curl.h>
#include <iostream>
#include <thread>
#include <chrono>

bool SeaweedService::upload(const std::vector<uint8_t>& data,
                            const std::string& filepath,
                            const std::string& extension) {
    // Validate inputs
    if (data.empty() || filepath.empty()) {
        std::cerr << "❌ Invalid upload parameters: data empty or no filepath" << std::endl;
        return false;
    }
    
    // Extract filename from path
    std::string filename = filepath.substr(filepath.find_last_of('/') + 1);
    if (filename.empty()) {
        filename = "file." + extension;
    }
    
    // Build URL
    std::string url = std::string(constants::SEAWEEDFS_FILER_URL) + filepath;
    
    // MIME type
    std::string mimeType = "image/" + extension;
    
    std::cout << "📤 Uploading to SeaweedFS: " << url << std::endl;
    std::cout << "📦 File size: " << data.size() << " bytes" << std::endl;
    
    // Try upload with retries
    for (int attempt = 1; attempt <= constants::MAX_RETRIES; attempt++) {
        if (uploadWithRetry(data, url, filename, mimeType, attempt)) {
            return true;
        }
        
        // Wait before retry (exponential backoff)
        if (attempt < constants::MAX_RETRIES) {
            std::this_thread::sleep_for(std::chrono::milliseconds(100 * attempt));
        }
    }
    
    std::cerr << "❌ Upload failed after " << constants::MAX_RETRIES << " attempts" << std::endl;
    return false;
}

bool SeaweedService::uploadWithRetry(const std::vector<uint8_t>& data,
                                     const std::string& url,
                                     const std::string& filename,
                                     const std::string& mimeType,
                                     int attempt) {
    CURL* curl = curl_easy_init();
    if (!curl) {
        std::cerr << "❌ Failed to initialize CURL" << std::endl;
        return false;
    }
    
    // Create multipart form
    curl_mime* mime = curl_mime_init(curl);
    curl_mimepart* part = curl_mime_addpart(mime);
    
    curl_mime_name(part, "file");
    curl_mime_data(part, reinterpret_cast<const char*>(data.data()), data.size());
    curl_mime_filename(part, filename.c_str());
    curl_mime_type(part, mimeType.c_str());
    
    // Configure CURL
    curl_easy_setopt(curl, CURLOPT_URL, url.c_str());
    curl_easy_setopt(curl, CURLOPT_MIMEPOST, mime);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, constants::CURL_TIMEOUT_SEC);
    
    // Execute
    CURLcode res = curl_easy_perform(curl);
    
    long responseCode = 0;
    bool success = false;
    
    if (res == CURLE_OK) {
        curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &responseCode);
        success = (responseCode == 200 || responseCode == 201);
        
        if (success) {
            std::cout << "✅ Uploaded to SeaweedFS (attempt " << attempt 
                      << "): HTTP " << responseCode << std::endl;
        } else {
            std::cerr << "❌ SeaweedFS returned HTTP " << responseCode 
                      << " (attempt " << attempt << ")" << std::endl;
        }
    } else {
        std::cerr << "❌ CURL error: " << curl_easy_strerror(res) 
                  << " (attempt " << attempt << ")" << std::endl;
    }
    
    curl_mime_free(mime);
    curl_easy_cleanup(curl);
    
    return success;
}