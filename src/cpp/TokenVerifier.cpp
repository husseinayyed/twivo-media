#include "TokenVerifier.hpp"
#include <jwt-cpp/jwt.h>
#include <chrono>
#include <iostream>
#include <string>
#include <stdexcept>  // For general exceptions

using namespace std;
using namespace sw::redis;

auto verifyUploadImageGetUserId(
    string_view tokenView, 
    const string& pubKey,
    Redis& redis
) -> string {
    try {
        const string token(tokenView);

        // Decode JWT
        auto decoded = jwt::decode(token);

        // Verify signature and standard claims
        auto verifier = jwt::verify()
            .allow_algorithm(jwt::algorithm::ed25519(pubKey, "", "", ""))  // EdDSA
            .with_issuer("twivo-backend")
            .with_audience("twivo-media");

        verifier.verify(decoded);
        
        // Check for jti claim
        if (decoded.has_payload_claim("jti")) {
            string jti = decoded.get_payload_claim("jti").as_string();
            
            // Check if jti exists in Redis
            try {
                auto exists = redis.exists(jti);
                if (exists) {
                    cerr << "JTI already exists - token already used: " << jti << endl;
                    return "";  // Token already used
                }
                
                // Store jti in Redis with expiration (match token expiration if available)
                if (decoded.has_payload_claim("exp")) {
                    auto exp = decoded.get_expires_at();
                    auto now = chrono::system_clock::now();
                    auto ttl = chrono::duration_cast<chrono::seconds>(exp - now).count();
                    
                    if (ttl > 0) {
                        redis.set(jti, "used");
                        redis.expire(jti, ttl);
                        cout << "JTI stored in Redis with TTL: " << ttl << " seconds" << endl;
                    } else {
                        return "";  // Token already expired
                    }
                } else {
                    // No expiration, store with default TTL (e.g., 1 hour)
                    redis.set(jti, "used");
                    redis.expire(jti, 3600);  // 1 hour default
                }
            } catch (const exception& e) {  // Catch any standard exception
                cerr << "Redis error while checking jti: " << e.what() << endl;
                // Fail closed - reject if Redis is down
                return "";
            } catch (...) {  // Catch any other exception
                cerr << "Unknown Redis error while checking jti" << endl;
                return "";
            }
        } else {
            // No jti claim - this token type is not allowed for upload
            cerr << "JWT missing jti claim - not allowed for upload" << endl;
            return "";
        }

        // Check custom payload: action
        if (decoded.has_payload_claim("action")) {
            string action = decoded.get_payload_claim("action").as_string();
            if (action != "uploadImage") {
                cerr << "Invalid action: " << action << endl;
                return "";  // invalid action
            }
        } else {
            cerr << "Missing action claim" << endl;
            return "";
        }

        // Check expiration
        if (decoded.has_payload_claim("exp")) {
            auto exp = decoded.get_expires_at();
            if (chrono::system_clock::now() > exp) {
                cerr << "Token expired" << endl;
                return "";
            }
        }

        // ✅ Return user ID (subject)
        return decoded.get_subject();

    } catch (const exception& e) {
        cerr << "JWT verification failed: " << e.what() << endl;
        return "";
    } catch (...) {
        cerr << "Unknown JWT verification failed" << endl;
        return "";
    }
}