## Resource Quotas (CPU & Memory)

This check tests if namespace resource quotas `CPU` and `memory` are under a specified threshold or percentage. Namespaces that utilize a lot of `CPU` or `memory` resources can sometimes run into an issue where controllers (_i.e. deployment or replica controllers_) are unable to schedule pods due to insufficient `CPU` or `memory`.

This check lists all namespaces in the cluster and checks if each resource (`CPU` and `memory`) are at an ok percentage.

This check can be configured to use either a `blacklist` or a `whitelist` of namespaces, allowing you to explicitly target or ignore specific namespaces. This can be changed with the environment variables `BLACKLIST` and `WHITELIST` which take `"true" or "false`. This check assumes the use of a `BLACKLIST` on check runs; if both `BLACKLIST` and `WHITELIST` options are enabled, `BLACKLIST` will take precedence. The default lists are:

    BLACKLIST = []string{"default"} // Ignores 'default' namespace by default.
    whitelist = []string{"kube-system", "kuberhealthy"} // Explicitly looks at 'kube-system' and 'kuberhealthy' namespaces by default.

Remember that the `BLACKLIST` option takes precedence and is on by default.

If any namespaces for the check need to be on the `blacklist` or `whitelist` they can be specified with the environment variable `NAMESPACES`, which expects a comma-separated list of namespaces (`"default,kube-system,istio"`) and can help you configure which namespaces to check when used in combination, with the `BLACKLIST` and `WHITELIST` environment variables.

Additionally, a `threshold` or `percentage` can be set that will determine when the check will configure and create alert messages. You can configure this value with the environment variable `THRESHOLD`, which expects a float value between `0.0` and `1.00` (_not inclusive_). By default, the threshold is set to `0.90` or `90%`

#### Check Steps

This check follows the list of actions in order during the run of the check:
1.  Lists all namespaces in the cluster.
2.  Sends a `go routine` for each namespace.
3.  Checks if used `CPU` and `memory` have reached the threshold.
4.  Creates errors for each violating namespace. (Up to two errors -- one for `CPU` and one for `memory`)

#### Check Details

- Namespace: kuberhealthy
- Check name: `resource-quota`
- Configurable check environment variables:
  - `BLACKLIST`: Initial container image. (default=`false` | _this check assumes BLACKLIST by default, even if this option is not enabled_)
  - `WHITELIST`: Container image to roll to. (default=`false`)
  - `NAMESPACES`: Name for the check's deployment. (_default for BLACKLIST_=`default` | _default for whitelist_=`kube-system,kuberhealthy`)
  - `THRESHOLD`: Name for the check's service. (default=`0.9`)
  - `CHECK_TIME_LIMIT`: Number of seconds the check will allow itself before timing out.
  - `DEBUG`: Turns on debug logging. (default=`false`)

#### Example KuberhealthyCheck Spec

The following configuration will create a deployment with 4 replicas and roll from `nginx:1.17-perl` to `nginx:1.17.5-perl`:

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: resource-quota
  namespace: kuberhealthy
spec:
  runInterval: 1h
  timeout: &resource_quota_check_timeout 2m
  podSpec:
    containers:
    - name: resource-quota
      image: quay.io/comcast/resource-quota-check:1.0.0
      imagePullPolicy: IfNotPresent
      env:
        - name: CHECK_TIME_LIMIT
          value: *resource_quota_check_timeout
      resources:
        requests:
          cpu: 15m
          memory: 15Mi
        limits:
          cpu: 30m
      restartPolicy: Never
    terminationGracePeriodSeconds: 30

```

#### Install

To use the *Deployment Check* with Kuberhealthy, apply the configuration file [resource-quota.yaml](resource-quota.yaml) to your Kubernetes Cluster. The following command will also apply the configuration file to your current context:

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/cmd/resource-quota-check/resource-quota-check.yaml`

Make sure you are using at least Kuberhealthy 2.0.0 or anything lateer. 

