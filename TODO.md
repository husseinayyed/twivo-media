# Twivo Media TODO

## Architecture

- [x] Remove OpenResty and Lua-based authentication.
      (Removed auth_request and _find_image subrequest from nginx.conf; Go now handles image delivery directly.)
- [x] Move JWT validation, authorization, JTI replay protection, etc. into the Go API.
      (Now handled directly in Go code.)
- [x] Keep Nginx as a simple reverse proxy and caching layer.
      (nginx.conf is now minimal: just proxy_pass and caching.)
- [x] Configure Nginx caching for image responses and suitable API responses.
      (Added proxy_cache, response_cache zone, and X-Cache-Status header.)
- [x] Add Nginx IP-based rate limiting, connection limits, request timeouts, and body-size limits.
      (Already present; unchanged – limit_req zones remain.)
- [x] Add circuit breakers for MongoDB, Redis, SeaweedFS, and imgproxy.
      (Breakers are initialized centrally and enabled around external dependency calls.)
- [x] Persist Redis data with a Docker volume.
      (Redis now mounts `redis_data:/data` in Docker Compose.)

## MongoDB

- [x] Add MongoDB to `docker-compose.yaml`.
- [x] Store file URLs, owners, tweet IDs, MIME types, sizes, dimensions, and SeaweedFS paths.
- [x] Store creation and update timestamps.
- [x] Add indexes for file ID, owner ID, tweet ID, SHA-256, and pHash.
      (Startup creates `nano_id`, `check_sum`, and `phash` indexes.)
- [x] Add MongoDB health checks and graceful startup handling.
      (MongoDB connection and startup index creation now verify the client with a ping and fail fast on setup errors.)

## Observability

- [x] Replace application and worker logging with structured zerolog events.
- [x] Add request start and completion logs with request ID, client IP, status, and duration.
- [x] Forward and return `X-Request-ID` through Nginx for request correlation.
- [x] Add structured Nginx access/error logs and bounded Docker log rotation.
- [x] Add Make targets for following and saving global, Nginx, and application logs.

## File Hashing

- [x] Calculate SHA-256 while streaming uploads.
      (Added hasher := sha256.New() + io.TeeReader in upload_route.go.)
- [x] Store SHA-256 checksum records for exact duplicate detection.
      (Stored in Redis under checksum keys; durable integrity metadata is still pending.)
- [ ] Calculate perceptual hashes (pHash) for images.
      (Not implemented.)
- [ ] Add tests for hash consistency and invalid image data.
      (Not added.)

## File Management

- [ ] Add authenticated file metadata endpoint.
      (Not implemented.)
- [ ] Add authenticated file listing endpoint.
      (Not implemented.)
- [ ] Add authenticated file deletion endpoint.
      (Not implemented.)
- [ ] Verify ownership before allowing deletion.
      (Not implemented as a separate endpoint.)
- [ ] Delete files from SeaweedFS and remove their MongoDB metadata.
      (SeaweedFS duplicate cleanup exists; MongoDB metadata deletion is pending.)
- [ ] Invalidate Redis, LRU, and Nginx cache entries after deletion.
      (Not implemented.)
- [ ] Add orphaned-file cleanup jobs.
      (Inline duplicate cleanup exists; durable cleanup jobs are pending.)
- [ ] Make deletion idempotent.
      (Not explicitly handled.)

## Caching

- [x] Add a bounded LRU cache for frequently requested metadata.
      (Already existed with size 100,000; now extended with BelongsTo, OwnerId, etc.)
- [ ] Add TTL support and cache invalidation.
      (Not added – LRU is size‑bounded with no TTL.)
- [x] Use Redis and MongoDB as fallbacks after LRU cache misses.
      (Redis is implemented and MongoDB fallback is included in the image route.)
- [ ] Track cache hits, misses, and evictions.
      (Only Nginx adds X‑Cache‑Status; internal LRU hit/miss not tracked.)
- [x] Configure Nginx to cache successful image responses.
      (Done – proxy_cache and proxy_cache_valid are set.)
- [x] Configure Nginx to cache image 404 responses.
      (Done – proxy_cache_valid 404 10m.)

## Cuckoo Filter

- [ ] Add a Cuckoo filter for fast file‑ID membership checks.
- [ ] Reject clearly unknown IDs before querying Redis or MongoDB.
- [ ] Always verify possible matches against Redis or MongoDB because filters can return false positives.
- [ ] Restore or rebuild the filter during application startup.
- [ ] Update the filter when files are created or deleted.
- [ ] Monitor capacity and insertion failures.
      (No Cuckoo filter work in this diff.)

## Abuse Protection and Reliability

- [ ] Add authenticated per‑user limits in Go.
- [x] Apply separate Nginx limits to uploads and image delivery.
      (Done – upload_zone and image_zone are configured in nginx.conf.)
- [ ] Add bounded worker queues and request timeouts.
- [ ] Add metrics for rejected requests, cache performance, backend failures, and filter saturation.
- [ ] Add integration tests for Redis, MongoDB, SeaweedFS, and Nginx.
- [ ] Use a CDN or WAF for infrastructure‑level DDoS protection.
      (None of these were touched.)

## Documentation

- [x] Update `README.md` after the OpenResty removal.
- [x] Document MongoDB environment variables.
- [x] Document upload and retrieval APIs.
- [x] Document cache behavior, including the ten-minute negative image cache.
- [ ] Document deletion APIs once file deletion is implemented.
- [ ] Add deployment and backup instructions.