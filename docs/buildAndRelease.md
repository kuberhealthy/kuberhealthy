# Build and Release

Kuberhealthy publishes multi-arch images for AMD64 and ARM64.

## Automatic builds

Builds publish to `docker.io/kuberhealthy/kuberhealthy`.
The Containerfile runs `go test` during the build.

## Release tags

Tags like `v1.2.3` publish `docker.io/kuberhealthy/kuberhealthy:<tag>`.
