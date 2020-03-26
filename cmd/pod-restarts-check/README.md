## Pod Restarts Check

The *Pod Restarts Check* checks for excessive pod restarts in a given `POD_NAMESPACE`. When the spec is applied to your
cluster, Kuberhealthy recognizes it as a KHCheck resource and provisions a checker pod to run the Pod Restarts Check.

The Pod Restarts Check deploys a pod that looks for pod resource events in a given `POD_NAMESPACE` and checks for
`Warning` event types with reason `BackOff`. If this specific event type count exceeds the `MAX_FAILURES_ALLOWED`, an
error is reporting back to Kuberhealthy.

In the example below, the check runs every 5m (spec.runInterval) with a check timeout set to 10 minutes (spec.timeout),
and a `MAX_FAILURES_ALLOWED` count set to 10. If the check does not complete within the given timeout it will report a
timeout error on the status page.

#### Pod Restarts Check Kube Spec:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-restarts
  namespace: kuberhealthy
spec:
  runInterval: 60m
  timeout: 62m
  podSpec:
    containers:
      - env:
          - name: POD_NAMESPACE
            value: "kube-system"
          - name: CHECK_POD_TIMEOUT
            value: "10m"
          - name: MAX_FAILURES_ALLOWED
            value: "10"
        image: kuberhealthy/pod-restarts-check:v2.1.1
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the Pod Restarts Check with Kuberhealthy, run:

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/2.0.0/cmd/pod-restarts-check/pod-restarts-check.yaml`

Make sure you are using the latest release of Kuberhealthy 2.0.0.
