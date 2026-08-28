
# Twivo Media

A Go-based media service for Twivo that securely receives image uploads, stores them in SeaweedFS, and serves resized WebP images through imgproxy.

## Features

- Go HTTP API using Gin
- JPEG, PNG, and WebP validation
- Image dimension validation
- Streaming uploads to SeaweedFS Filer
- Redis-backed upload task scheduling with Asynq
- JWT authentication using an Ed25519 public key
- JWT issuer, audience, and JTI replay validation
- Redis-backed rate limiting and JTI tracking
- Image resizing and WebP conversion through imgproxy
- Nginx/OpenResty reverse proxy and caching
- Nmap user-agent blocking

## Architecture

```text
Client
  │
  ▼
OpenResty Gateway
  ├── /upload ──► JWT validation ──► Twivo API
  ├── /ping ───────────────────────► Twivo API
  └── /i/{nanoid} ─► Redis metadata ─► imgproxy ─► SeaweedFS

Twivo API ──► Redis / Asynq
           └─► SeaweedFS Filer
        
```

## Technology Stack

- **Language:** Go
- **Web framework:** Gin
- **Reverse proxy:** OpenResty / Nginx with Lua
- **Task queue:** Asynq
- **Cache and metadata:** Redis
- **Storage:** SeaweedFS
- **Image processing:** imgproxy
- **Deployment:** Docker Compose

## Requirements

- Docker
- Docker Compose
- A JWT Ed25519 public key
- Valid JWT issuer and audience configuration

## Configuration

Create a `.env` file in the project root:

```dotenv
REDIS_URL=redis:6379
SEAWEEDFS_FILER_URL=http://weed-filer:8888
WEED_FILER_URL=http://weed-filer:8888
PUBLIC_KEY_PATH=/absolute/path/to/public.pem
JWT_ISS=your-jwt-issuer
JWT_AUD=your-jwt-audience
```

The JWT used for uploads must contain:

- `sub` — user ID
- `id` — tweet ID
- `iss` — configured issuer
- `aud` — configured audience
- `jti` — unique token ID

## Running the Project

Start the services:

```bash
docker compose up --build
```

Stop the services:

```bash
docker compose down
```

Remove persistent SeaweedFS volumes:

```bash
docker compose down -v
```

The gateway is available at:

```text
http://localhost:80
```

## HTTP Routes

### Health Check

```http
GET /ping
```

### Upload Media

```http
POST /upload
x-twivo-backend: <signed-jwt>
Content-Type: image/jpeg
```

The gateway validates the JWT and forwards the verified claims as:

```http
X-User-ID: <jwt.sub>
X-Tweet-ID: <jwt.id>
```

Uploads are limited to 10 MB at the gateway and are streamed to SeaweedFS.

### Serve Resized Images

```http
GET /i/{nanoid}
```

The nanoid is resolved through Redis. The corresponding file is fetched from SeaweedFS and resized to WebP by imgproxy.

## Service Ports

| Service | Port |
|---|---:|
| OpenResty gateway | `80` |
| SeaweedFS master | `9333` |
| SeaweedFS volume | `8085` |
| SeaweedFS filer | `8888` |
| imgproxy | `8000` |
| Twivo API | `8020` |

## Project Structure

```text
twivo-media/
├── internal/
│   ├── database/redis/   # Redis connection
│   ├── storage/          # SeaweedFS upload and deletion
│   ├── tasks/            # Asynq task definitions
│   └── worker/           # Background task processing
├── main.go               # Gin API and upload handling
├── nginx.conf            # OpenResty gateway configuration
├── docker-compose.yaml   # Development services
├── Dockerfile
├── gateway.Dockerfile
├── go.mod
└── README.md
```

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPLv3).
