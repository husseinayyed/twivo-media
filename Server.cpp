// src/Server.cpp
#include "Server.hpp"
#include "handlers/UploadHandler.hpp"
#include "services/RedisService.hpp"
#include "core/Constants.hpp"
#include <uwebsockets/App.h>
#include <iostream>
#include <csignal>
#include <atomic>

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
        auto& redis = RedisService::getInstance();
        
        if (redis.ping()) {
            res->writeStatus("200 OK")
               ->writeHeader("Content-Type", "application/json")
               ->end(R"({"status":"healthy","service":"media","redis":"connected"})");
        } else {
            res->writeStatus("503 Service Unavailable")
               ->writeHeader("Content-Type", "application/json")
               ->end(R"({"status":"unhealthy","service":"media","redis":"disconnected"})");
        }
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