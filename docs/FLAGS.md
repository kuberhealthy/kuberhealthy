Available flags for use in Kuberhealthy

# Flags

|Flag|Description|Optional|Default|
|---|---|---|---|
|`-kubecfg`|Absolute path to a kube config file.|Yes| `$HOME/.kube/config`|
|`-listenAddress`|The port kuberhealthy will listen on.|Yes| `8080`|
|`-componentStatusChecks`|Bool to enable/disable Kuberhealthy's [master component](https://kubernetes.io/docs/concepts/overview/components/#master-components) status [check](https://github.com/Comcast/kuberhealthy/blob/master/README.md#component-health).|Yes|`True`|
|`-daemonsetChecks`|Bool to enable/disable Kuberhealthy's test daemon set [check](https://github.com/Comcast/kuberhealthy/blob/master/README.md#daemonset-deployment-and-termination).|Yes|`True`|
|`-podRestartChecks`|Bool to enable/disable Kuberhealthy's pod restart check [check](https://github.com/Comcast/kuberhealthy/blob/master/README.md#excessive-pod-restarts).|Yes|`True`|
|`-podStatusChecks`|Bool to enable/disable Kuberhealthy's pod status check [check](https://github.com/Comcast/kuberhealthy/blob/master/README.md#pod-status).|Yes|`True`|
|`-forceMaster`|Bool to enable/disable election and force master mode.  Useful/Intended for local testing.|Yes|`False`|
|`-debug`|Bool to enable/disable debug logging.|Yes|`False`|
|`dsPauseContainerImageOverride`|Set an alternate image location for the pause container the daemon set checker uses for its daemon set configuration.|Yes|`gcr.io/google_containers/pause:0.8.0`|
|`podCheckNamespaces`|A comma separated list of namespaces in which to check for pod statuses and restart counts.|Yes|`kube-system`|
