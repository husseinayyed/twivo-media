// src/Server.hpp
#pragma once

#include <memory>
#include <string>

class UploadHandler;

class MediaServer {
public:
    MediaServer();
    ~MediaServer();
    
    /**
     * Start the server
     * @return true if started successfully
     */
    bool start();
    
    /**
     * Stop the server gracefully
     */
    void stop();
    
private:
    void setupRoutes();
    std::string loadPublicKey();
    bool initializeServices();
    
    std::unique_ptr<UploadHandler> uploadHandler_;
    std::string publicKey_;
    bool isRunning_;
};