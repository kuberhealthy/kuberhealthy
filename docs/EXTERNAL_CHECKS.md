### External Checks

External checks are pods that Kuberhealthy spins up to run any image the user specifies.  Checks are specified via custom resources (khcheckcrd package) that look like this:


```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: test-check
  namespace: kuberhealthy
spec:
  CurrentUUID: 390922a4-0573-4fff-b14d-a0d7f63cc73e
  PodSpec:
    containers: null // TODO
  RunInterval: 600000000000 // TODO
```

External check pods are injected with the following environment variables reguardless of the `PodSpec` set by the user:

- `KH_CHECK_NAME` - The name of the check that spawned this pod.  Normally the name of the khstate custom resource.
- `KH_RUN_UUID` - A unique ID for the specific check run instance.  Used in validating that the calling pod IP is currently allowed to report status.
- `KH_REPORTING_URL` - The URL that pods should send a GET request back to.  The body should be a `health.State` struct in JSON like below:

```json
// TODO
```

Checks can be written in anything as long as its turned into a docker image.  The spec for the pod that needs run should be set in the yaml above.  The pod should then
send its status json payload to the exact URL in the `KH_REPORTING_URL` environment variable.




#### TODO 

- run interval should be in seconds
- test that loads spec from CRD and runs entire check to completion
- monitoring of khcheck custom resource changes
- creation of checks from khcheck resources
- updating of checks from khcheck resources
- write client under pkg/checks/external/client
- create framework getting started project for people to fork
- more docs and walkthroughs
- add khcheck custom resource to helm chart

