# Build and Release

Kuberhealthy publishes multi-arch images for AMD64 and ARM64.

## Automatic builds

Pushes to `main` run `build-push-image.yml` and publish to `docker.io/kuberhealthy/kuberhealthy`.
Tags include `<branch>-<short-sha>` and `main`.

## Release tags

Tags like `v1.2.3` publish `docker.io/kuberhealthy/kuberhealthy:<tag>`.
