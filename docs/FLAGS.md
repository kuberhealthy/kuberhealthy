Available flags for use in Kuberhealthy

# Flags

|Flag|Description|Optional|Default|
|---|---|---|---|
|`--kubecfg`|Absolute path to a kube config file.|Yes| `$HOME/.kube/config`|
|`--listenAddress`|The port kuberhealthy will listen on.|Yes| `8080`|
|`--forceMaster`|Bool to enable/disable election and force master mode.  Useful/Intended for local testing.|Yes|`False`|
|`--debug`|Bool to enable/disable debug logging.|Yes|`False`|
