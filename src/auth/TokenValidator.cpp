#include "TokenValidator.hpp"
#include "Services/RedisService.hpp"
#include <jwt-cpp/jwt.h>
#include <jwt-cpp/traits/nlohmann-json/traits.h>
#include <chrono>
#include <iostream>

using json_traits = jwt::traits::nlohmann_json;

bool TokenValidator::verify(const std::string& token,
                            const std::string& publicKey,
                            std::string& userId,
                            std::string& tweetId) {
    try {
        auto decoded = jwt::decode<json_traits>(token);
        
        auto verifier = jwt::verify<jwt::default_clock, json_traits>(jwt::default_clock{})
            .allow_algorithm(jwt::algorithm::ed25519(publicKey, "", "", ""))
            .with_issuer("twivo-backend")
            .with_audience("twivo-media");
        
        verifier.verify(decoded);
        
        if (!decoded.has_payload_claim("jti")) {
            std::cerr << "Missing jti claim" << std::endl;
            return false;
        }
        
        std::string jti = decoded.get_payload_claim("jti").as_string();
        auto& redis = RedisService::getInstance();
        
        if (redis.jtiExists(jti)) {
            std::cerr << "JTI already used" << std::endl;
            return false;
        }
        
        long long ttl = 3600;
        if (decoded.has_payload_claim("exp")) {
            auto exp = decoded.get_expires_at();
            auto now = std::chrono::system_clock::now();
            ttl = std::chrono::duration_cast<std::chrono::seconds>(exp - now).count();
        }
        
        redis.storeJTI(jti, ttl);
        
        if (!decoded.has_payload_claim("action") || 
            decoded.get_payload_claim("action").as_string() != "uploadImage") {
            return false;
        }
        
        userId = decoded.get_subject();
        tweetId = decoded.get_payload_claim("id").as_string();
        
        return true;
        
    } catch (const std::exception& e) {
        std::cerr << "JWT validation error: " << e.what() << std::endl;
        return false;
    }
}