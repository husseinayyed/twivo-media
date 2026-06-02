// src/handlers/UploadHandler.hpp
#pragma once

#include "models/UploadContext.hpp"
#include <string>
#include <memory>

// Forward declarations
class TokenValidator;
class SeaweedService;

class UploadHandler {
public:
    UploadHandler();
    ~UploadHandler();
    
    /**
     * Handle incoming upload request
     * @param res uWebSockets response object
     * @param req uWebSockets request object
     * @param publicKey Public key for JWT verification
     */
    void handle(auto* res, auto* req, const std::string& publicKey);
    
private:
    // Process the uploaded image data
    bool processImage(UploadContextPtr ctx, auto* res);
    
    // Validate image dimensions
    bool validateDimensions(UploadContextPtr ctx);
    
    // Upload to SeaweedFS and save metadata
    bool saveToStorage(UploadContextPtr ctx, auto* res);
    
    // Extract JWT claims
    bool authenticate(const std::string& token, 
                      const std::string& publicKey,
                      UploadContextPtr ctx);
    
    std::unique_ptr<TokenValidator> tokenValidator_;
    std::unique_ptr<SeaweedService> seaweedService_;
};