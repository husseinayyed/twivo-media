# --- Builder Stage (The "heavy lifting" stage) ---
FROM ubuntu:24.04 AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential cmake git curl unzip zip tar pkg-config libssl-dev \
    && rm -rf /var/lib/apt/lists/*

# Install vcpkg
RUN git clone https://github.com/Microsoft/vcpkg.git /opt/vcpkg && \
    /opt/vcpkg/bootstrap-vcpkg.sh

WORKDIR /app
COPY vcpkg.json ./
RUN /opt/vcpkg/vcpkg install --triplet x64-linux

# Copy source (This invalidates cache only when source code changes)
COPY CMakeLists.txt ./
COPY src ./src/
COPY include ./include/
COPY main.cpp ./

# Build the application
RUN mkdir build && cd build && \
    cmake .. \
    -DCMAKE_TOOLCHAIN_FILE=/opt/vcpkg/scripts/buildsystems/vcpkg.cmake \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=OFF && \
    cmake --build . --config Release -j$(nproc)

# --- Development Stage ---
# This inherits everything from builder, keeping build tools and vcpkg intact.
FROM builder AS dev
# Keeps the container running for 'docker-compose exec' or 'docker exec'
CMD ["tail", "-f", "/dev/null"]

# --- Runtime Stage (Small) ---
FROM ubuntu:24.04 AS prod

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/build/app .
RUN useradd -m -u 1001 appuser && chown -R appuser:appuser /app
USER appuser

CMD ["./app"]