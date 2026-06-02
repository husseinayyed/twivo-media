// src/utils/CryptoUtils.hpp
#pragma once

#include <string>
#include <vector>

namespace crypto {

    /**
     * Calculate SHA256 hash of binary data
     * @param data Input binary data
     * @return Hexadecimal string of SHA256 hash (64 characters)
     */
    std::string sha256(const std::vector<uint8_t>& data);
    
    /**
     * Generate a cryptographically random 12-character ID (like nanoId)
     * @return Random alphanumeric string of length 12
     */
    std::string generateNanoId();
    
} // namespace crypto