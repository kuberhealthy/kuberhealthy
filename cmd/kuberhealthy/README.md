# Building Locally

If you don't have `just` or `podman` installed, install them.

- To run tests, use `just build`
- To build locally, run `just build`
- The image will build as `kuberhealthy:localdev`
- To deploy Kuberhealthy on a Kubernetes cluster, apply the Kubernetes flat file spec to your cluster with `just install`
- To run your image on the target Kubernetes cluster, use `just deploy`
