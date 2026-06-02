// src/services/RedisService.hpp
#pragma once

#include <hiredis/hiredis.h>
#include <string>
#include <vector>

class RedisService {
public:
    static RedisService& getInstance();
    
    // Initialize connection
    bool connect(const std::string& url);
    
    // Close connection
    ~RedisService();
    
    // Check connection health
    bool ping();
    
    // JTI operations (for token replay prevention)
    bool storeJTI(const std::string& jti, long long ttl_seconds);
    bool jtiExists(const std::string& jti);
    
    // Stream operations (for metadata)
    bool addToStream(const std::string& stream, 
                     const std::vector<std::pair<std::string, std::string>>& fields);
    
    // Get raw context (for compatibility)
    redisContext* getContext() { return ctx_; }
    
private:
    RedisService() = default;
    
    // Disable copy
    RedisService(const RedisService&) = delete;
    RedisService& operator=(const RedisService&) = delete;
    
    redisContext* ctx_ = nullptr;
};