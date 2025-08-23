# online-shop-image

A lightweight Go microservice for processing, transforming, and storing images on local disk or S3-compatible object storage. Designed to be embedded in an e‑commerce stack, the service provides HTTP endpoints for uploading images, generating thumbnails, validating formats/sizes, and persisting files with configurable storage backends.

## Table of Contents

* [Features](#features)
* [Contributing](#contributing)

  * [Pull request guidelines](#pull-request-guidelines)
  * [Running tests](#running-tests)
  * [Code style](#code-style)
* [Testing](#testing)

  * [Unit tests](#unit-tests)
  * [Integration tests (MinIO/S3)](#integration-tests-minios3)
* [Roadmap / TODO](#roadmap--todo)

## Features

* Pluggable storage backends: local disk and S3-compatible object storage (e.g., AWS S3, MinIO).
* Automatic image validation (allowed types, max size) and optional resizing / thumbnail generation.
* Safe filename / path generation and content-addressable storage option (optional).
* Configurable via environment variables or config file.


```

## Contributing

Thanks for considering contributing! We welcome bug reports, feature requests, and pull requests.

### Pull request guidelines

1. Fork the repository and create a feature branch: `git checkout -b feat/your-feature`.
2. Write tests for new features/bug fixes.
3. Keep changes focused and atomic.
4. Update or add documentation where appropriate.
5. Open a PR against `main` with a clear title and description, linking any relevant issues.

### Running tests

Run unit tests:

```bash
go test ./...
```

For more exhaustive testing, run with verbose output:

```bash
go test ./... -v
```

If your project uses test coverage, generate a coverage report:

```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Code style

* Follow the Go formatting and linting conventions: `gofmt`/`go fmt`, `golangci-lint`.
* Use idiomatic Go: small functions, clear error handling, context propagation (`context.Context`).
* Keep public APIs stable and well-documented with Go doc comments.

## Testing

### Unit tests

* Keep unit tests fast and hermetic.
* Mock external dependencies (S3) where possible using interfaces.

### Integration tests (MinIO/S3)

Use Docker Compose with MinIO to run integration tests locally. Example `docker-compose.yml` snippet:

```yaml
version: '3.7'
services:
  minio:
    image: minio/minio
    command: server /data
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
    volumes:
      - minio-data:/data
volumes:
  minio-data:
```

Set env vars to point to the MinIO instance during test runs and run your integration test suite. Consider using `testcontainers` or ephemeral MinIO instances for CI.

## Roadmap / TODO

* Add image optimization (progressive JPEG, WebP conversion).
* Add content-addressable storage mode (store by SHA256 hash).
* Add signed temporary URLs for private content.
* Add rate limiting and request size throttling.
* Add thumbnail cache invalidation strategies and lifecycle management.
* Provide Helm chart for Kubernetes deployment.

---

*Generated README template for `online-shop_image`. Please adapt any command names, endpoints, and env var names to match the actual implementation in your repo.*
