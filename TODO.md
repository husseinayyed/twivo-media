# Twivo Media TODO

## Architecture

- [ ] Remove OpenResty and Lua-based authentication.
- [ ] Move JWT validation, authorization, JTI replay protection, etc. into the Go API.
- [ ] Keep Nginx as a simple reverse proxy and caching layer.
- [ ] Configure Nginx caching for image responses and suitable API responses.
- [ ] Add Nginx IP-based rate limiting, connection limits, request timeouts, and body-size limits.

## MongoDB

- [ ] Add MongoDB to `docker-compose.yaml`.
- [ ] Store file URLs, owners, tweet IDs, MIME types, sizes, dimensions, and SeaweedFS paths.
- [ ] Store creation, update, and deletion timestamps.
- [ ] Add indexes for file ID, owner ID, tweet ID, SHA-256, and pHash.
- [ ] Add MongoDB health checks and graceful startup handling.

## File Hashing

- [ ] Calculate SHA-256 while streaming uploads.
- [ ] Store SHA-256 for integrity checks and exact duplicate detection.
- [ ] Calculate perceptual hashes (pHash) for images.
- [ ] Use pHash to find visually similar images.
- [ ] Add tests for hash consistency and invalid image data.

## File Management

- [ ] Add authenticated file metadata endpoint.
- [ ] Add authenticated file listing endpoint.
- [ ] Add authenticated file deletion endpoint.
- [ ] Verify ownership before allowing deletion.
- [ ] Delete files from SeaweedFS and remove their MongoDB metadata.
- [ ] Invalidate Redis, LRU, and Nginx cache entries after deletion.
- [ ] Add orphaned-file cleanup jobs.
- [ ] Make deletion idempotent.

## Caching

- [ ] Add a bounded LRU cache for frequently requested metadata.
- [ ] Add TTL support and cache invalidation.
- [ ] Use Redis and MongoDB as fallbacks after LRU cache misses.
- [ ] Track cache hits, misses, and evictions.
- [ ] Configure Nginx to cache successful image responses.

## Cuckoo Filter

- [ ] Add a Cuckoo filter for fast file-ID membership checks.
- [ ] Reject clearly unknown IDs before querying Redis or MongoDB.
- [ ] Always verify possible matches against Redis or MongoDB because filters can return false positives.
- [ ] Restore or rebuild the filter during application startup.
- [ ] Update the filter when files are created or deleted.
- [ ] Monitor capacity and insertion failures.

## Abuse Protection and Reliability

- [ ] Add authenticated per-user limits in Go.
- [ ] Apply separate limits to uploads and image delivery.
- [ ] Add bounded worker queues and request timeouts.
- [ ] Add metrics for rejected requests, cache performance, backend failures, and filter saturation.
- [ ] Add integration tests for Redis, MongoDB, SeaweedFS, and Nginx.
- [ ] Use a CDN or WAF for infrastructure-level DDoS protection.

## Documentation

- [ ] Update `README.md` after the OpenResty removal.
- [ ] Document MongoDB environment variables.
- [ ] Document upload, retrieval, and deletion APIs.
- [ ] Document cache invalidation behavior.
- [ ] Add deployment and backup instructions.