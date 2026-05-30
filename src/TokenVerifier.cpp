#include "TokenVerifier.hpp"
#include <chrono>
#include <iostream>

using namespace std::chrono;
std::optional<jwt::decoded_jwt<json_traits>> verifyUploadImageGetUserId(
    std::string_view token_view, 
    const std::string& pubKey, 
    sw::redis::Redis& redis
){
    std::string token_str(token_view);
    
    try {
        // Get decoded directly from decode
        auto decoded = jwt::decode<json_traits>(token_str);
        
        auto verifier = jwt::verify<jwt::default_clock, json_traits>(jwt::default_clock{})
            .allow_algorithm(jwt::algorithm::ed25519(pubKey, "", "", ""))
            .with_issuer("twivo-backend")
            .with_audience("twivo-media");
        
        verifier.verify(decoded);

        // Check for required claims
        if (!decoded.has_payload_claim("jti")) {
            std::cerr << "Missing jti claim" << std::endl;
            return std::nullopt;
        }
        
        std::string jti = decoded.get_payload_claim("jti").as_string();

        if (redis.exists(jti)) {
            std::cerr << "JTI already used: " << jti << std::endl;
            return std::nullopt;
        }

        auto exp = decoded.has_payload_claim("exp") ? 
                   decoded.get_expires_at() : 
                   (system_clock::now() + hours(1));
        
        auto ttl = duration_cast<seconds>(exp - system_clock::now()).count();
        
        if (ttl <= 0) {
            std::cerr << "Token expired" << std::endl;
            return std::nullopt;
        }
        
        redis.set(jti, "used");
        redis.expire(jti, ttl);

        if (!decoded.has_payload_claim("action")) {
            std::cerr << "Missing action claim" << std::endl;
            return std::nullopt;
        }
        
        if (decoded.get_payload_claim("action").as_string() != "uploadImage") {
            std::cerr << "Invalid action: " << decoded.get_payload_claim("action").as_string() << std::endl;
            return std::nullopt;
        }

        return decoded;
        
    } catch (const std::exception& e) {
        std::cerr << "JWT operation failed: " << e.what() << std::endl;
        return std::nullopt;
    }
}