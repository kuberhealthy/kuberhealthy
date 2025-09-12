# Local Development

Use these steps to work on Kuberhealthy locally.

1. Build and format the code with `gopls` and `gofmt`. Editors that integrate with `gopls` usually run `go fmt` for you, or run `go fmt ./...` manually.
2. Run `just kind` to start a [kind](https://kind.sigs.k8s.io/) cluster, build the Kuberhealthy image, and load it into the cluster.
3. In another terminal, run `just browse` to port-forward to the cluster and view Kuberhealthy.
4. Make changes to the code. When ready to test them, focus the `just kind` terminal and press **Enter** to rebuild the image and redeploy it.
5. Apply checks to the cluster, such as `kubectl apply -f tests/khcheck-test.yaml`.

