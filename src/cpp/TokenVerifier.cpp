#include "TokenVerifier.hpp"
#include <jwt-cpp/jwt.h>
#include <chrono>
#include <iostream>
#include <string>
#include <optional>

using namespace std;
using namespace sw::redis;

std::optional<jwt::decoded_jwt<jwt::traits::kazuho_picojson>>
verifyUploadImageGetUserId(
    string_view tokenView,
    const string& pubKey,
    Redis& redis
) {
    const string token(tokenView);

    // Decode JWT – store in optional because we can't default-construct
    std::optional<jwt::decoded_jwt<jwt::traits::kazuho_picojson>> decodedOpt;
    try {
        decodedOpt = jwt::decode(token);          // decode returns a decoded_jwt
    } catch (const exception& e) {
        cerr << "JWT decode failed: " << e.what() << endl;
        return nullopt;
    }

    // Now we can safely dereference – it always contains a value after the try block
    auto& decoded = *decodedOpt;

    // Verify signature and standard claims
    try {
        auto verifier = jwt::verify()
            .allow_algorithm(jwt::algorithm::ed25519(pubKey, "", "", ""))
            .with_issuer("twivo-backend")
            .with_audience("twivo-media");
        verifier.verify(decoded);
    } catch (const exception& e) {
        cerr << "JWT verification failed: " << e.what() << endl;
        return nullopt;
    }

    // Check jti claim
    if (!decoded.has_payload_claim("jti")) {
        cerr << "JWT missing jti claim – not allowed for upload" << endl;
        return nullopt;
    }

    string jti = decoded.get_payload_claim("jti").as_string();

    // Redis existence check
    try {
        if (redis.exists(jti)) {
            cerr << "JTI already exists – token already used: " << jti << endl;
            return nullopt;
        }

        // Store jti with TTL
        if (decoded.has_payload_claim("exp")) {
            auto exp = decoded.get_expires_at();
            auto now = chrono::system_clock::now();
            auto ttl = chrono::duration_cast<chrono::seconds>(exp - now).count();
            if (ttl <= 0) {
                cerr << "Token expired" << endl;
                return nullopt;
            }
            redis.set(jti, "used");
            redis.expire(jti, ttl);
        } else {
            // Default TTL 1 hour
            redis.set(jti, "used");
            redis.expire(jti, 3600);
        }
    } catch (const exception& e) {
        cerr << "Redis error: " << e.what() << endl;
        return nullopt;
    }

    // Check action claim
    if (!decoded.has_payload_claim("action")) {
        cerr << "Missing action claim" << endl;
        return nullopt;
    }
    string action = decoded.get_payload_claim("action").as_string();
    if (action != "uploadImage") {
        cerr << "Invalid action: " << action << endl;
        return nullopt;
    }

    // Expiration check (already covered by verifier, but double-check)
    if (decoded.has_payload_claim("exp") && chrono::system_clock::now() > decoded.get_expires_at()) {
        cerr << "Token expired" << endl;
        return nullopt;
    }

    // Success: return the decoded token
    return decoded;
}