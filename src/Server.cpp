// src/Server.cpp
#define REDISPTR_GLOBAL
#include "Server.hpp"
#include "handlers/UploadHandler.hpp"
#include "Services/RedisService.hpp"
#include "Core/Constants.hpp"
#include <uwebsockets/App.h>
#include <curl/curl.h>
#include <ctime>
#include <iostream>
#include <fstream>
#include <csignal>
#include <atomic>
static sw::redis::Redis redis(std::getenv("REDIS_URL"));
sw::redis::Redis* redisPtr = &redis;

// Global flag for graceful shutdown
static std::atomic<bool> g_running{true};
static uWS::App* g_app = nullptr;

void signalHandler(int signal) {
    std::cout << "\n🛑 Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
    
    // uWebSockets doesn't have a direct stop, but we can trigger exit
    // The event loop will exit when no more connections are active
    if (g_app) {
        // Close all listening sockets (workaround)
        // In practice, the process will exit when main returns
    }
}

MediaServer::MediaServer() 
    : uploadHandler_(std::make_unique<UploadHandler>())
    , isRunning_(false) {
}

MediaServer::~MediaServer() = default;

bool MediaServer::initializeServices() {
    // Get Redis URL from environment
    const char* redis_url = std::getenv("REDIS_URL");
    if (!redis_url) {
        redis_url = "tcp://redis:6379"; // Default for Docker
    }
    
    auto& redis = RedisService::getInstance();
    if (!redis.connect(redis_url)) {
        std::cerr << "❌ Failed to connect to Redis" << std::endl;
        return false;
    }
    
    std::cout << "✅ Services initialized" << std::endl;
    return true;
}

std::string MediaServer::loadPublicKey() {
    const char* cert_path = std::getenv("SSL_CERT_PATH");
    std::string path = cert_path ? cert_path : constants::DEFAULT_PUBLIC_KEY_PATH;
    
    std::ifstream file(path, std::ios::binary);
    if (!file.is_open()) {
        std::cerr << "❌ Failed to open public key: " << path << std::endl;
        return "";
    }
    
    std::string key((std::istreambuf_iterator<char>(file)), 
                     std::istreambuf_iterator<char>());
    
    if (key.empty() || key.find("-----BEGIN PUBLIC KEY-----") == std::string::npos) {
        std::cerr << "❌ Invalid public key format" << std::endl;
        return "";
    }
    
    std::cout << "✅ Public key loaded from: " << path << std::endl;
    return key;
}

void MediaServer::setupRoutes() {
    // Note: This is called from start() with the app instance
    // The actual routing is set up in the start method
}

bool MediaServer::start() {
    // Setup signal handlers for graceful shutdown
    signal(SIGINT, signalHandler);
    signal(SIGTERM, signalHandler);
    
    // Initialize services
    if (!initializeServices()) {
        return false;
    }
    
    // Load public key
    publicKey_ = loadPublicKey();
    if (publicKey_.empty()) {
        return false;
    }
    
    // Create uWebSockets app
    uWS::App app;
    g_app = &app;
    
    // Health check endpoint
    app.get("/health", [](auto* res, auto* /*req*/) {
    extern sw::redis::Redis* redisPtr;
    bool redis_ok = false;
    bool seaweed_ok = false;
    
    try {
        redisPtr->ping();
        redis_ok = true;
    } catch (...) {}
    
    // Quick check if filer responds
    CURL* curl = curl_easy_init();
    if (curl) {
        curl_easy_setopt(curl, CURLOPT_URL, "http://weed-filer:8888/");
        curl_easy_setopt(curl, CURLOPT_TIMEOUT, 2L);
        curl_easy_setopt(curl, CURLOPT_NOBODY, 1L);
        seaweed_ok = (curl_easy_perform(curl) == CURLE_OK);
        curl_easy_cleanup(curl);
    }
    
    bool healthy = redis_ok && seaweed_ok;
    
    std::string response = "{\"status\":\"" + std::string(healthy ? "healthy" : "unhealthy") + "\"}";
    res->writeStatus(healthy ? "200 OK" : "503 Service Unavailable")
       ->writeHeader("Content-Type", "application/json")
       ->end(response);
});
    
    // Root endpoint
    app.get("/", [](auto* res, auto* /*req*/) {
        res->writeStatus("200 OK")
           ->writeHeader("Content-Type", "text/plain")
           ->end("Twivo Media Service\n\nPOST /upload - Upload images\nGET /health - Health check");
    });
    
    // Upload endpoint
    app.post("/upload", [this](auto* res, auto* req) {
        uploadHandler_->handle(res, req, publicKey_);
    });
    
    // Start listening
    app.listen(constants::PORT, [](auto* listenSocket) {
        if (listenSocket) {
            std::cout << "🚀 Media server running on port " << constants::PORT << std::endl;
            std::cout << "   Upload endpoint: POST /upload" << std::endl;
            std::cout << "   Health check: GET /health" << std::endl;
        } else {
            std::cerr << "❌ Failed to bind to port " << constants::PORT << std::endl;
        }
    });
    
    isRunning_ = true;
    std::cout << "✅ Server started, waiting for requests..." << std::endl;
    
    // Run the event loop (blocks until server stops)
    app.run();
    
    isRunning_ = false;
    g_app = nullptr;
    
    std::cout << "👋 Server shutdown complete" << std::endl;
    return true;
}

void MediaServer::stop() {
    if (isRunning_) {
        std::cout << "Stopping server..." << std::endl;
        g_running = false;
        // uWebSockets will exit the event loop when appropriate
    }
}