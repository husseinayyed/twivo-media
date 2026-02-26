# Twivo Media Processor 🖼️✨

**[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)**
**[![C++](https://img.shields.io/badge/C%2B%2B-20-blue.svg)](https://isocpp.org/std/the-standard)**
**[![Redis](https://img.shields.io/badge/Redis-4.0-red.svg)](https://redis.io/)**
**[![Serialization](https://img.shields.io/badge/Serialization-Cap'n_Proto-orange.svg)](https://capnproto.org/)**

A high-performance, secure media processing service built specifically for the **Twivo-backend** project, optimized for zero-copy data transfer.

---

## 🚀 Overview

Twivo Media Processor is a **modern C++20 service** designed to handle image uploads with absolute minimal CPU overhead. 

By utilizing **Cap'n Proto over Raw TCP**, this service achieves near-zero serialization latency, ensuring that every CPU cycle is dedicated to image processing rather than data parsing.

It provides:
- **Zero-Copy Communication** via Cap'n Proto RPC
- **Raw TCP Transport** for maximum throughput and minimal overhead
- **JWT-based Authorization** (Ed25519)
- **Redis-backed Rate Limiting**
- **Intelligent Image Resizing** and automatic WebP conversion

---

## ✨ Key Features

✅ **Zero-Copy Architecture** – Data is read directly from the buffer without parsing 
✅ **Extreme Efficiency** – Optimized for high-scale environments (Millions of requests/sec)
✅ **JWT Authentication** – Secure Ed25519 signature verification  
✅ **Rate Limiting** – Redis-backed request throttling  
✅ **Advanced Image Processing** – Powered by `stb_image` and `libwebp`
✅ **C++20 Native** – Built with modern language features and RAII  

---

## 🛠️ Tech Stack

| Category | Tools / Libraries |
|----------|------------------|
| **Language** | C++20 |
| **Serialization** | Cap'n Proto (Zero-copy) |
| **Transport** | Raw TCP Sockets |
| **Redis Client** | Redis++ |
| **JWT** | jwt-cpp (Ed25519) |
| **Image Engine** | stb_image & libwebp |

---

## 📋 System Requirements

- **C++20-compatible compiler** (GCC 11+, Clang 13+, or MSVC 2022+)
- **Redis** v4.0+
- **Cap'n Proto Compiler** (`capnp`)
- **OpenSSL** & **libwebp**
- Linux / macOS

---

## 📦 Installation

### Prerequisites (Ubuntu/Debian)

```bash
sudo apt-get install build-essential cmake libssl-dev libwebp-dev redis-server capnproto libcapnp-dev

Build
git clone https://github.com/husseinayued/twivo-media.git
cd twivo-media

mkdir build && cd build
cmake -DCMAKE_CXX_STANDARD=20 ..
make

🧱 Project Structure
twivo-media/
├── include/           # Header files (.hpp)
│   └── cpp/
│       ├── ImageProcessor.hpp
│       ├── TokenVerifier.hpp
│       └── ...
├── src/               # Source files (.cpp)
│   └── cpp/
│       ├── ImageProcessor.cpp
│       ├── TokenVerifier.cpp
│       └── main.cpp
├── schema/            # Cap'n Proto definitions
│   └── media.capnp
├── CMakeLists.txt
└── README.md
```
---

# Author
- **husseinayyed**

---
# 📜 License

This project is licensed under the GNU Affero General Public License v3.0 (AGPLv3).
If you modify and deploy this software over a network, you must make the source code of your modified version available under the same license.
Built to power Twivo’s media pipeline with every drop of CPU performance 🚀
