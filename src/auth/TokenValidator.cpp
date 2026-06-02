// src/auth/TokenValidator.cpp
#include "TokenValidator.hpp"
#include "services/RedisService.hpp"
#include <jwt-cpp/jwt.h>
#include <jwt-cpp/traits/picojson/traits.h>
#include <chrono>
#include <iostream>

// Define the traits type (using picojson)
using json_traits = jwt::traits::picojson;

// Forward declare the decoded_jwt type
template class jwt::decoded_jwt<json_traits>;

bool TokenValidator::verify(const std::string& token,
                            const std::string& publicKey,
                            std::string& userId,
                            std::string& tweetId) {
    auto decodedOpt = verifyAndDecode(token, publicKey);
    
    if (!decodedOpt.has_value()) {
        return false;
    }
    
    return extractClaims(decodedOpt.value(), userId, tweetId);
}

std::optional<jwt::decoded_jwt<json_traits>>
TokenValidator::verifyAndDecode(const std::string& token, const std::string& publicKey) {
    try {
        // Step 1: Decode the token (doesn't verify signature yet)
        auto decoded = jwt::decode<json_traits>(token);
        
        // Step 2: Create verifier with Ed25519 algorithm
        auto verifier = jwt::verify<jwt::default_clock, json_traits>(jwt::default_clock{})
            .allow_algorithm(jwt::algorithm::ed25519(publicKey, "", "", ""))
            .with_issuer("twivo-backend")
            .with_audience("twivo-media");
        
        // Step 3: Verify signature and claims
        verifier.verify(decoded);
        
        // Step 4: Check JTI (replay prevention)
        if (!decoded.has_payload_claim("jti")) {
            std::cerr << "JWT missing jti claim" << std::endl;
            return std::nullopt;
        }
        
        std::string jti = decoded.get_payload_claim("jti").as_string();
        
        auto& redis = RedisService::getInstance();
        
        // Check if token was already used
        if (redis.jtiExists(jti)) {
            std::cerr << "JWT already used (replay attack detected): " << jti << std::endl;
            return std::nullopt;
        }
        
        // Calculate TTL for JTI storage
        long long ttl_seconds = 3600; // Default 1 hour
        
        if (decoded.has_payload_claim("exp")) {
            auto exp = decoded.get_expires_at();
            auto now = std::chrono::system_clock::now();
            ttl_seconds = std::chrono::duration_cast<std::chrono::seconds>(exp - now).count();
            
            if (ttl_seconds <= 0) {
                std::cerr << "Token already expired" << std::endl;
                return std::nullopt;
            }
        }
        
        // Store JTI to prevent replay
        if (!redis.storeJTI(jti, ttl_seconds)) {
            std::cerr << "Failed to store JTI in Redis" << std::endl;
            return std::nullopt;
        }
        
        // Step 5: Check action claim
        if (!decoded.has_payload_claim("action")) {
            std::cerr << "JWT missing action claim" << std::endl;
            return std::nullopt;
        }
        
        std::string action = decoded.get_payload_claim("action").as_string();
        if (action != "uploadImage") {
            std::cerr << "Invalid action: " << action << " (expected uploadImage)" << std::endl;
            return std::nullopt;
        }
        
        return decoded;
        
    } catch (const std::exception& e) {
        std::cerr << "JWT validation failed: " << e.what() << std::endl;
        return std::nullopt;
    }
}

bool TokenValidator::extractClaims(const jwt::decoded_jwt<json_traits>& decoded,
                                   std::string& userId,
                                   std::string& tweetId) {
    // Extract subject (user ID)
    if (!decoded.has_subject()) {
        std::cerr << "JWT missing sub claim" << std::endl;
        return false;
    }
    userId = decoded.get_subject();
    
    // Extract custom id claim (tweet ID)
    if (!decoded.has_payload_claim("id")) {
        std::cerr << "JWT missing id claim" << std::endl;
        return false;
    }
    
    try {
        tweetId = decoded.get_payload_claim("id").as_string();
    } catch (const std::bad_cast& e) {
        std::cerr << "Invalid type for id claim: " << e.what() << std::endl;
        return false;
    }
    
    std::cout << "✅ Token validated for user: " << userId << ", tweet: " << tweetId << std::endl;
    return true;
}