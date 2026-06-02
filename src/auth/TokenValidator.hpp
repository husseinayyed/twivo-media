// src/auth/TokenValidator.hpp
#pragma once

#include <optional>
#include <string>

// Forward declaration to avoid including jwt-cpp in header
namespace jwt {
    template<typename traits>
    class decoded_jwt;
}

// Forward declare traits (will be defined in cpp)
struct picojson_traits;

class TokenValidator {
public:
    /**
     * Verify JWT token and extract user information
     * @param token JWT token string
     * @param publicKey Ed25519 public key in PEM format
     * @param userId Output: user ID from 'sub' claim
     * @param tweetId Output: tweet ID from 'id' claim
     * @return true if token is valid and claims are present
     */
    bool verify(const std::string& token, 
                const std::string& publicKey,
                std::string& userId,
                std::string& tweetId);
    
    /**
     * Verify token and return the decoded object (for advanced use)
     */
    std::optional<jwt::decoded_jwt<picojson_traits>> 
    verifyAndDecode(const std::string& token, const std::string& publicKey);
    
private:
    bool extractClaims(const jwt::decoded_jwt<picojson_traits>& decoded,
                       std::string& userId,
                       std::string& tweetId);
};