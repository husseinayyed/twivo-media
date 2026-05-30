cmake_minimum_required(VERSION 3.22.1)
project(TwivoMedia LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)
# --- 1. Dependencies ---
find_package(OpenSSL REQUIRED)
find_package(hiredis CONFIG REQUIRED)
find_package(redis++ CONFIG REQUIRED)
find_package(unofficial-uwebsockets CONFIG REQUIRED)
find_package(CURL REQUIRED)
find_package(jwt-cpp CONFIG REQUIRED)
find_package(nlohmann_json CONFIG REQUIRED)

get_target_property(REDISPLUSPLUS_INCLUDE_DIR redis++::redis++_static INTERFACE_INCLUDE_DIRECTORIES)

# --- 2. Build Internal Library ---
add_library(TokenImageVerifier STATIC 
    src/TokenVerifier.cpp
    src/ImageType.cpp
)

# Force the directory directly into the target's search list
target_include_directories(TokenImageVerifier PUBLIC 
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${REDISPLUSPLUS_INCLUDE_DIR}
    ${NLOHMANN_JSON_INCLUDE_DIR}
)

target_link_libraries(TokenImageVerifier
    PUBLIC OpenSSL::SSL
    PUBLIC OpenSSL::Crypto
    PUBLIC jwt-cpp::jwt-cpp
    PRIVATE redis++::redis++_static
    PRIVATE hiredis::hiredis
)

# --- 3. Main Application ---
add_executable(app main.cpp)
target_compile_options(app PRIVATE -march=native -O3 -Wall -Wextra)

target_link_libraries(app
    PRIVATE TokenImageVerifier
    PRIVATE unofficial::uwebsockets::uwebsockets
    PRIVATE CURL::libcurl
)