#pragma once

#include <jwt-cpp/jwt.h>
#include <jwt-cpp/traits/nlohmann-json/traits.h>  // Use nlohmann-json traits
#include <sw/redis++/redis++.h>
#include <optional>
#include <string_view>
#include <string>

// Use nlohmann-json traits - this works with vcpkg!
using json_traits = jwt::traits::nlohmann_json;

std::optional<jwt::decoded_jwt<json_traits>> verifyUploadImageGetUserId(
    std::string_view token, 
    const std::string& pubKey, 
    sw::redis::Redis& redis
);