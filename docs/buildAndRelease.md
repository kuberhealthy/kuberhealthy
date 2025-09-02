# Build and Release Process

This project publishes multi-architecture container images for AMD64 and ARM64.

## Automatic Image Builds

Changes merged to the `main` or `v3` branches that touch the `cmd`, `internal`, or `pkg` directories trigger a build of the Kuberhealthy image. The workflow builds for AMD64 and ARM64 and pushes an image tagged with `<branch>-<short-sha>` to GitHub Container Registry.

## Cutting a Release

Kuberhealthy releases are driven by Git tags that follow [semantic versioning](https://semver.org/).

1. Ensure the desired code is merged to `main`.
2. Create and push a tag like `v1.2.3`.

Tag creation updates installation instructions and Kustomize specs to reference the new tag and publishes a multi-architecture image to Docker Hub tagged with the same version.

That is all that is required to release a new version of Kuberhealthy.
