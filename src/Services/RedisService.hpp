// src/services/RedisService.hpp
#pragma once

#include <hiredis/hiredis.h>
#include <string>
#include <vector>

class RedisService {
public:
    static RedisService& getInstance();
    
    bool connect(const std::string& url);
    bool ping();
    bool storeJTI(const std::string& jti, long long ttl_seconds);
    bool jtiExists(const std::string& jti);
    bool addToStream(const std::string& stream, 
                     const std::vector<std::pair<std::string, std::string>>& fields);
    
    ~RedisService();  // Only ONE destructor declaration
    
private:
    RedisService() = default;
    
    // Disable copy
    RedisService(const RedisService&) = delete;
    RedisService& operator=(const RedisService&) = delete;
    
    redisContext* ctx_ = nullptr;
};