// src/services/RedisService.cpp
#include "RedisService.hpp"
#include <iostream>
#include <cstring>

RedisService& RedisService::getInstance() {
    static RedisService instance;
    return instance;
}

RedisService::~RedisService() {
    if (ctx_) {
        redisFree(ctx_);
        ctx_ = nullptr;
    }
}

bool RedisService::connect(const std::string& url) {
    // Parse Redis URL (format: tcp://host:port)
    std::string host = "127.0.0.1";
    int port = 6379;
    
    // Simple URL parsing (you can expand this)
    if (url.find("tcp://") == 0) {
        std::string rest = url.substr(6);
        size_t colon = rest.find(':');
        if (colon != std::string::npos) {
            host = rest.substr(0, colon);
            port = std::stoi(rest.substr(colon + 1));
        }
    }
    
    // Connect with timeout (2 seconds)
    struct timeval timeout = {2, 0};
    ctx_ = redisConnectWithTimeout(host.c_str(), port, timeout);
    
    if (!ctx_ || ctx_->err) {
        if (ctx_) {
            std::cerr << "Redis connection error: " << ctx_->errstr << std::endl;
            redisFree(ctx_);
            ctx_ = nullptr;
        } else {
            std::cerr << "Redis connection allocation failed" << std::endl;
        }
        return false;
    }
    
    std::cout << "✅ Redis connected to " << host << ":" << port << std::endl;
    return true;
}

bool RedisService::ping() {
    if (!ctx_) return false;
    
    redisReply* reply = (redisReply*)redisCommand(ctx_, "PING");
    if (!reply) return false;
    
    bool ok = (reply->type == REDIS_REPLY_STRING && std::string(reply->str) == "PONG");
    freeReplyObject(reply);
    return ok;
}

bool RedisService::storeJTI(const std::string& jti, long long ttl_seconds) {
    if (!ctx_) return false;
    
    redisReply* reply = (redisReply*)redisCommand(ctx_, 
        "SETEX %s %lld used", 
        jti.c_str(), 
        ttl_seconds);
    
    if (!reply) return false;
    
    bool ok = (reply->type == REDIS_REPLY_STATUS && std::string(reply->str) == "OK");
    freeReplyObject(reply);
    return ok;
}

bool RedisService::jtiExists(const std::string& jti) {
    if (!ctx_) return false;
    
    redisReply* reply = (redisReply*)redisCommand(ctx_, "EXISTS %s", jti.c_str());
    if (!reply) return false;
    
    bool exists = (reply->type == REDIS_REPLY_INTEGER && reply->integer == 1);
    freeReplyObject(reply);
    return exists;
}

bool RedisService::addToStream(const std::string& stream,
                               const std::vector<std::pair<std::string, std::string>>& fields) {
    if (!ctx_) return false;
    
    // Build XADD command
    std::vector<const char*> argv;
    std::vector<size_t> argvlen;
    
    argv.push_back("XADD");
    argvlen.push_back(4);
    
    argv.push_back(stream.c_str());
    argvlen.push_back(stream.length());
    
    argv.push_back("*");
    argvlen.push_back(1);
    
    for (const auto& field : fields) {
        argv.push_back(field.first.c_str());
        argvlen.push_back(field.first.length());
        argv.push_back(field.second.c_str());
        argvlen.push_back(field.second.length());
    }
    
    redisReply* reply = (redisReply*)redisCommandArgv(ctx_, argv.size(), argv.data(), argvlen.data());
    if (!reply) return false;
    
    bool ok = (reply->type == REDIS_REPLY_STRING);
    freeReplyObject(reply);
    return ok;
}