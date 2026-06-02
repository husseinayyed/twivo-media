// src/utils/CryptoUtils.cpp
#include "CryptoUtils.hpp"
#include <openssl/evp.h>
#include <sstream>
#include <iomanip>
#include <random>
#include <iostream>

namespace crypto {

    std::string sha256(const std::vector<uint8_t>& data) {
        EVP_MD_CTX* ctx = EVP_MD_CTX_new();
        if (!ctx) {
            std::cerr << "Failed to create EVP context" << std::endl;
            return "";
        }
        
        unsigned char hash[EVP_MAX_MD_SIZE];
        unsigned int hash_len = 0;
        
        // Initialize SHA256
        if (EVP_DigestInit_ex(ctx, EVP_sha256(), nullptr) != 1) {
            std::cerr << "SHA256 init failed" << std::endl;
            EVP_MD_CTX_free(ctx);
            return "";
        }
        
        // Update with data
        if (EVP_DigestUpdate(ctx, data.data(), data.size()) != 1) {
            std::cerr << "SHA256 update failed" << std::endl;
            EVP_MD_CTX_free(ctx);
            return "";
        }
        
        // Finalize
        if (EVP_DigestFinal_ex(ctx, hash, &hash_len) != 1) {
            std::cerr << "SHA256 finalize failed" << std::endl;
            EVP_MD_CTX_free(ctx);
            return "";
        }
        
        EVP_MD_CTX_free(ctx);
        
        // Convert to hex string
        std::stringstream ss;
        for (unsigned int i = 0; i < hash_len; i++) {
            ss << std::hex << std::setw(2) << std::setfill('0') 
               << static_cast<int>(hash[i]);
        }
        
        return ss.str();
    }
    
    std::string generateNanoId() {
        // URL-safe alphabet (no special characters)
        const std::string alphabet = 
            "0123456789"
            "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
            "abcdefghijklmnopqrstuvwxyz";
        
        const size_t length = 12;
        
        // Use cryptographically secure random
        std::random_device rd;
        std::mt19937_64 generator(rd());
        std::uniform_int_distribution<> distribution(0, alphabet.size() - 1);
        
        std::string nanoId;
        nanoId.reserve(length);
        
        for (size_t i = 0; i < length; ++i) {
            nanoId += alphabet[distribution(generator)];
        }
        
        return nanoId;
    }
    
} // namespace crypto