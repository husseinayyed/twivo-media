#include "RedisService.hpp"
#include <iostream>
#include <cstring>

// Static instance getter
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
    std::string host = "redis";
    int port = 6379;
    
    if (url.find("tcp://") == 0) {
        std::string rest = url.substr(6);
        size_t colon = rest.find(':');
        if (colon != std::string::npos) {
            host = rest.substr(0, colon);
            port = std::stoi(rest.substr(colon + 1));
        }
    }
    
    struct timeval timeout = {2, 0};
    ctx_ = redisConnectWithTimeout(host.c_str(), port, timeout);
    
    if (!ctx_ || ctx_->err) {
        if (ctx_) {
            std::cerr << "Redis error: " << ctx_->errstr << std::endl;
            redisFree(ctx_);
            ctx_ = nullptr;
        }
        return false;
    }
    
    std::cout << "✅ Redis connected to " << host << ":" << port << std::endl;
    return true;
}

bool RedisService::ping() {
    if (!ctx_) {
        std::cerr << "Redis context is null" << std::endl;
        return false;
    }
    
    redisReply* reply = (redisReply*)redisCommand(ctx_, "PING");
    if (!reply) {
        std::cerr << "Redis PING returned null reply" << std::endl;
        return false;
    }
    
    bool ok = false;
    if (reply->type == REDIS_REPLY_STRING) {
        ok = (strcmp(reply->str, "PONG") == 0);
    }
    
    freeReplyObject(reply);
    return ok;
}

bool RedisService::storeJTI(const std::string& jti, long long ttl_seconds) {
    if (!ctx_) return false;
    
    redisReply* reply = (redisReply*)redisCommand(ctx_, "SETEX %s %lld used", jti.c_str(), ttl_seconds);
    if (!reply) return false;
    
    bool ok = (reply->type == REDIS_REPLY_STATUS);
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
bool RedisService::storeNanoId(const std::string& id, const std::string& path, long long ttl_seconds) {
    if (!ctx_) return false;
    
     redisReply* reply = (redisReply*)redisCommand(ctx_, "SETEX %s %lld %s", 
                                                id.c_str(), 
                                                ttl_seconds, 
                                                path.c_str());
    
    if (!reply) {
        std::cerr << "Redis SETEX error: " << ctx_->errstr << std::endl;
        return false;
    }
    
    bool ok = (reply->type == REDIS_REPLY_STATUS);
    freeReplyObject(reply);
    return ok;
}
bool RedisService::addToStream(const std::string& stream,
                               const std::vector<std::pair<std::string, std::string>>& fields) {
    if (!ctx_) return false;
    
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