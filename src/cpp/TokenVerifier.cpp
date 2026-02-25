#include "TokenVerifier.hpp"
#include <jwt-cpp/jwt.h>
#include <chrono>
#include <iostream>
#include <string>

using namespace std;

auto verifyUploadImageGetUserId(string_view tokenView, const string& pubKey) -> string {
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

        // Check custom payload: action
        if (decoded.get_payload_claim("action").as_string() != "uploadImage") {
            return "";  // invalid action
        }

        // Check expiration
        if (decoded.has_payload_claim("exp")) {
            auto exp = decoded.get_expires_at();
            if (chrono::system_clock::now() > exp) return "";
        }

        // ✅ Return user ID (subject)
        return decoded.get_subject();

    } catch (const exception& e) {
        cerr << "JWT verification failed: " << e.what() << endl;
        return "";
    }
}