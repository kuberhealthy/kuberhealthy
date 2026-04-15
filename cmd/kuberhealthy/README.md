# Building Locally

- Install `just` and `podman` first.
- Run `just -l` from this directory to see the available commands.

## Just Commands

- `just test`: run unit tests
- `just build`: build the local image
- `just run`: run the controller locally
- `just kustomize`: apply the Kustomize manifests

## Configuration

- Controller configuration is documented in [docs/FLAGS.md](../../docs/FLAGS.md).
- Deployment and local-run examples are documented in [docs/DEPLOYING_KUBERHEALTHY.MD](../../docs/DEPLOYING_KUBERHEALTHY.MD).
