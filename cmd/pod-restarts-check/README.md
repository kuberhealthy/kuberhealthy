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
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - env:
          - name: POD_NAMESPACE
            value: "kube-system"
          - name: MAX_FAILURES_ALLOWED
            value: "10"
        image: kuberhealthy/pod-restarts-check:v2.5.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```
#### Options

By default, `Pod Restarts Check` will check pods in the same namespace it is installed into.  This means the RBAC requirements for the service account the check runs with can be limited to a single namespace scope.

It is possible to configure `Pod Restarts Check` to check pods from all namespaces in a cluster, this requires cluster wide permissions for the service account and is not recommended for multi-tenant setups.

#### How-to

##### kubectl apply

To implement the Pod Restarts Check with Kuberhealthy, run:

`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/2.0.0/cmd/pod-restarts-check/pod-restarts-check.yaml`


If you want to enable the cluster wide option described above then __instead__ apply with cluster permissions [pod-restarts-check-clusterscope.yaml](pod-restarts-check-clusterscope.yaml).

##### Helm

```
helm repo add kuberhealthy https://kuberhealthy.github.io/kuberhealthy/helm-repos
helm install kuberhealthy kuberhealthy/kuberhealthy --set check.podRestarts.enabled=true
```

To enable cluster wide check with cluster permissions
```
helm install kuberhealthy kuberhealthy/kuberhealthy --set check.podRestarts.enabled=true --set check.podRestarts.allNamespaces=true
```
