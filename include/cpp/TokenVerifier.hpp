#include <optional>
#include <jwt-cpp/jwt.h>
#include <string_view>
#include <sw/redis++/redis++.h>
std::optional<jwt::decoded_jwt<jwt::traits::kazuho_picojson>>
verifyUploadImageGetUserId(std::string_view token, const std::string& pubKey, sw::redis::Redis& redis);