# Build stage
FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y \
    build-essential \
    cmake \
    git \
    curl \
    unzip \
    zip \
    tar \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Install vcpkg
WORKDIR /opt
RUN git clone https://github.com/Microsoft/vcpkg.git
WORKDIR /opt/vcpkg
RUN ./bootstrap-vcpkg.sh

# Build app
WORKDIR /app

# Copy vcpkg manifest first for better caching
COPY vcpkg.json ./

# Install dependencies
RUN /opt/vcpkg/vcpkg install --triplet x64-linux

# Copy source files
COPY CMakeLists.txt ./
COPY src ./src/
COPY include ./include/
COPY external ./external/
COPY main.cpp ./

# Build the application statically
RUN mkdir build && cd build && \
    cmake .. \
    -DCMAKE_TOOLCHAIN_FILE=/opt/vcpkg/scripts/buildsystems/vcpkg.cmake \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF && \
    cmake --build . --config Release -j$(nproc)

# Runtime stage (small)
FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy ONLY the binary (no libraries needed for static build)
COPY --from=builder /app/build/app .
# Create non-root user for security
RUN useradd -m -u 1001 appuser && chown -R appuser:appuser /app
USER appuser

CMD ["./app"]