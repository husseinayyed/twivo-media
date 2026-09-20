# Contributing to Twivo Media

Thanks for contributing to Twivo Media.

## Development Setup

1. Create a local `.env` file with the required service and JWT settings.
2. Generate local Ed25519 keys when needed:

   ```bash
   ./scripts/generate_keys.sh
   ```

3. Start the development containers:

   ```bash
   make dev
   ```

4. Open a shell in the idle media container:

   ```bash
   make dev-shell
   ```

5. Start the API and worker manually inside the container:

   ```bash
   go run .
   ```

The development container is intentionally idle until `go run .` is started manually.

## Useful Commands

```bash
make dev        # Build and start development services
make dev-shell  # Open a shell in the media container
make dev-down   # Stop development services
make dev-clean  # Stop services and remove development volumes

go test ./...   # Run tests
gofmt -w .      # Format Go files before committing
```

## Code Changes

- Keep changes focused and consistent with the existing package structure.
- Add or update tests for behavioral changes.
- Run `gofmt` on changed Go files.
- Do not commit `.env`, private keys, passwords, tokens, or generated service data.
- Keep API, Docker, and README documentation aligned with behavior changes.

## Pull Requests

Include the following in the pull request description:

- What changed and why.
- How the change was tested.
- Any configuration, migration, or deployment steps required.
- Known limitations or follow-up work.

Before requesting review, verify that tests pass and that the development Compose stack starts successfully with `make dev`.
