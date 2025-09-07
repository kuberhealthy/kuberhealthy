## Legal

- If you would like to contribute code to this project you can do so through GitHub by forking the repository and sending a pull request.
- Your commits [must be signed](https://probot.github.io/apps/dco/) so that the DCO bot will accept them.  This means using `git commit -s` when comitting.

## Developer Contribution Workflow

- If you are making a large change, you should submit a proposal issue first so that you don't risk your feature being denied
- Fork the Kuberhealthy/Kuberhealthy repository on github 
- Clone your fork to your machine
- Create a branch describing your change
- Develop your feature and push changes to a branch in your fork
- Add your name to the `CONTRIBUTORS.md` file
- Add your company to the `ADOPTERS.md` file (optional)
- Open a pull request from your branch branch in your fork to the master branch on github.com/kuberhealthy/kuberhealthy
- Wait for a project maintainer to address your change and merge your code.  Keep an eye on your open PR.
- Celebrate! 🎉

## General Requirements

- The code must be formatted with `go fmt`
- The change must include tests for new functionality created
- The code must pass all Github CI tests

## Tooling

- Kuberhealthy uses `just` instead of `make`
- Kuberhealthy uses `podman` instead of `docker`

## Just Commands

The repository includes a `Justfile` for common local development tasks. Install `just` and ensure required CLIs (`podman`, `kubectl`, `kustomize`, and `kind` where noted) are available in your PATH.

- `just build`: Build the Kuberhealthy container image using Podman.
- `just kind`: Create a local KIND cluster and deploy Kuberhealthy for development. Requires `kind`, `kubectl`, `kustomize`, and `podman`. After the initial deployment, press **Enter** to rebuild the image and redeploy it to the cluster. Press **Ctrl-C** to cancel and tear down the cluster.
- `just kind-clean`: Delete the local KIND cluster. Use this if you want a fresh cluster or to clean up when `just kind` is not running.
- `just test`: Run unit tests for `internal/...` and `cmd/...` packages.
- `just run`: Build and run Kuberhealthy locally with useful defaults (`KH_LOG_LEVEL=debug`, `KH_EXTERNAL_REPORTING_URL=localhost:80`, `POD_NAMESPACE=kuberhealthy`, `POD_NAME=kuberhealthy-test`).
- `just kustomize`: Apply the Kubernetes manifests in `deploy/` using `kustomize build | kubectl apply -f -`.
- `just browse`: Port-forward the `kuberhealthy` service in namespace `kuberhealthy` to `localhost:8080` and open a browser. Press Ctrl-C to stop the port-forward. Override port with `PORT=9090 just browse`.
