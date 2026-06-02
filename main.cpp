// src/main.cpp
#include "src/Server.hpp"
#include <curl/curl.h>
#include <iostream>

int main() {
    std::cout << "=== Twivo Media Service ===" << std::endl;
    std::cout << "Starting up..." << std::endl;
    
    // Initialize CURL globally (required for HTTPS, but fine for HTTP too)
    CURLcode curl_init = curl_global_init(CURL_GLOBAL_DEFAULT);
    if (curl_init != CURLE_OK) {
        std::cerr << "❌ Failed to initialize CURL: " << curl_easy_strerror(curl_init) << std::endl;
        return 1;
    }
    
    // Create and start server
    MediaServer server;
    bool success = server.start();
    
    // Cleanup CURL
    curl_global_cleanup();
    
    return success ? 0 : 1;
}