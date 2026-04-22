cmake_minimum_required(VERSION 3.22.1)
project(TwivoMedia LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Find OpenSSL (required for JWT)
find_package(OpenSSL REQUIRED)

# Find packages using vcpkg
find_package(hiredis CONFIG REQUIRED)
find_package(redis++ CONFIG REQUIRED)
find_package(unofficial-uwebsockets CONFIG REQUIRED)
find_package(WebP CONFIG REQUIRED)

# Get redis++ include directories
get_target_property(REDISPLUSPLUS_INCLUDE_DIR redis++::redis++_static INTERFACE_INCLUDE_DIRECTORIES)

# Source files
set(TOKEN_IMAGE_SOURCES
    src/cpp/TokenVerifier.cpp
)

set(IMAGE_SOURCES
    src/cpp/ImageType.cpp
    src/cpp/ImageProcessor.cpp
)

# Static libraries
add_library(TokenImageVerifier STATIC ${TOKEN_IMAGE_SOURCES})
add_library(Image STATIC ${IMAGE_SOURCES})

# Include directories for TokenImageVerifier
target_include_directories(TokenImageVerifier
    PUBLIC external/include
    PUBLIC include/cpp
    PRIVATE ${REDISPLUSPLUS_INCLUDE_DIR}  # Add redis++ include
)

# Include directories for Image
target_include_directories(Image
    PUBLIC include/cpp/stbimages
    PUBLIC include/c/stbimages
)

# Link libraries for TokenImageVerifier (needs OpenSSL and redis++)
target_link_libraries(TokenImageVerifier
    PUBLIC OpenSSL::SSL
    PUBLIC OpenSSL::Crypto
    PRIVATE redis++::redis++_static  # Link redis++
    PRIVATE hiredis::hiredis
)

# Link libraries for Image
target_link_libraries(Image
    PUBLIC WebP::webp
    PUBLIC WebP::webpdecoder
)

# Main executable
add_executable(app main.cpp)

# Add compile options
target_compile_options(app PRIVATE -march=native -O3)

# Add include directories for app
target_include_directories(app PRIVATE 
    include/cpp 
    include/c 
    ${CMAKE_CURRENT_SOURCE_DIR}/external/uuid_v4
    /home/husseinayyed/vcpkg/packages/uwebsockets_x64-linux/include
    ${REDISPLUSPLUS_INCLUDE_DIR}  # Add redis++ include for app
)

# Link libraries for app
target_link_libraries(app
    PRIVATE Image
    PRIVATE TokenImageVerifier
    PRIVATE unofficial::uwebsockets::uwebsockets
    PRIVATE redis++::redis++_static
    PRIVATE hiredis::hiredis
    PRIVATE OpenSSL::SSL
    PRIVATE OpenSSL::Crypto
)