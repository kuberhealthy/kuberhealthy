# Contributing

Thanks for helping Kuberhealthy. We welcome early adopters and focused PRs.

## Legal

- Fork the repo and open a PR.
- Sign commits with DCO (`git commit -s`).

## Workflow

1. Open an issue for larger changes.
2. Create a branch for your work.
3. Keep changes scoped and add tests when behavior changes.
4. Update docs if user-facing behavior changes.
5. Add yourself to `CONTRIBUTORS.md`.
6. Optionally add your org to `ADOPTERS.md`.
7. Open a PR against `main`.

## Requirements

- Format Go code with `go fmt`.
- Ensure tests pass.

## Tooling

- `just` is used for common tasks.
- `podman` is the container engine used in scripts.

## Just commands

- `just build`: Build the Kuberhealthy image with Podman.
- `just kind`: Create a KIND cluster and deploy Kuberhealthy.
- `just kind-clean`: Delete the KIND cluster.
- `just test`: Run unit tests for `internal/...` and `cmd/...`.
- `just run`: Build and run locally with dev defaults.
- `just kustomize`: Apply manifests from `deploy/kustomize/`.
- `just browse`: Port-forward the service and open a browser.
