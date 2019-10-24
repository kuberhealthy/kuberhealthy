### External Checks

External checks are configured using `khcheck` custom resources.  These `khchecks` can create pods from any Kuberhealthy check image the user specifies.  Pods are created in the namespace that their `khcheck` was placed into.  A list of pre-made checks that you can easily enable are listed [in the external checks registry](../docs/EXTERNAL_CHECKS_REGISTRY.md).  

As soon as your `khcheck` resource is applied to the cluster, Kuberhealthy will begin running it.  If a change is made, Kuberhealthy will shut down any active checks gracefully and restart them with the updated configuration.

### `khcheck` Anatomy

A `khcheck` looks like this:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-test-check # the name of this check and the checker pod
  namespace: kuberhealthy # the namespace the checker pod will run in
spec:
  runInterval: 30s # The interval that Kuberhealthy will run your check on 
  timeout: 2m # After this much time, Kuberhealthy will kill your check and consider it "failed"
  extraAnnotations: # Optional extra annotations your pod can have
    comcast.com/testAnnotation: test.annotation
  extraLabels: # Optional extra labels your pod can be configured with
    testLabel: testLabel
  podSpec: # The exact pod spec that will run.  All normal pod spec is valid here.
    containers:
    - env: # Environment variables are optional but a recommended way to configure check behavior
      - name: REPORT_FAILURE
        value: "false"
      - name: REPORT_DELAY
        value: 6s
      image: quay.io/comcast/test-external-check:latest # The image of the check you want to run.
      imagePullPolicy: Always # During check development, it helps to set this to 'Always' to prevent on-node image caching.
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```

### Visualized

Here is an illustration of how Kuberhealthy runs checks each in their own pod.  In this example, the checker pod both deploys a daemonset and tears it down while carefully watching for errors.  The result of the check is then sent back to Kuberhealthy and channeled into upstream metrics and status pages to indicate basic Kubernetes cluster functionality across all nodes in a cluster.

<img src="../images/kh-ds-check.gif">

### Creating Your Own Checks

To learn how to write your own checks of any kind, check out the [documentation for it here](../docs/EXTERNAL_CHECK_CREATION.md).

