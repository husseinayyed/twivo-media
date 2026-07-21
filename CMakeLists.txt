cmake_minimum_required(VERSION 3.22.1)
project(TwivoMedia LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

find_package(OpenSSL REQUIRED)
find_package(hiredis CONFIG REQUIRED)
find_package(redis++ CONFIG REQUIRED)
find_package(unofficial-uwebsockets CONFIG REQUIRED)
find_package(CURL REQUIRED)
find_package(nlohmann_json CONFIG REQUIRED)

set(SOURCES
    main.cpp
    src/ImageType.cpp
    src/Server.cpp
    src/Utils/CryptoUtils.cpp
    src/Utils/ImageUtils.cpp
    src/Services/RedisService.cpp
    src/Services/SeaweedService.cpp
    src/handlers/UploadHandler.cpp
)

add_executable(app ${SOURCES})

target_include_directories(app PRIVATE 
    include
    src
    src/Core
    src/Services
    src/Utils
)

target_link_libraries(app
    PRIVATE unofficial::uwebsockets::uwebsockets
    PRIVATE CURL::libcurl
    PRIVATE OpenSSL::SSL OpenSSL::Crypto
    PRIVATE redis++::redis++_static hiredis::hiredis
    PRIVATE nlohmann_json::nlohmann_json
)