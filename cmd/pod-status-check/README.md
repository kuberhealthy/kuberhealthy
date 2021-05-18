## Pod Status Check

The `Pod Status Check` checks for pods older than ten minutes and are in an unhealthy lifecycle phase.  If a
`podStatusCheck` detects that a pod is down, an alert is shown on the status page. When a pod is found to be in error,
the exact pod's name will be shown as one of the `Error` field's strings.


#### Example Pod Status KuberhealtyCheck Spec
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-status
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 15m
  podSpec:
    containers:
      - env:
          - name: SKIP_DURATION # the duration of time that pods are ignored for after being created
            value: "10m"
          - name: TARGET_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        image: kuberhealthy/pod-status-check:v1.3.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### Pod Status Phases
`Phases that this check considers healthy`
- Running:  The Pod has been bound to a node, and all of the Containers have been created. At least one Container is still running, or is in the process of starting or restarting.
- Succeeded:  All Containers in the Pod have terminated in success, and will not be restarted.

`Phases that this check considers unhealthy`
- Pending:  The Pod has been accepted by the Kubernetes system, but one or more of the Container images has not been created. This includes time before being scheduled as well as time spent downloading images over the network, which could take a while.

Note: This check assumes that a pod is unhealthy if it is over 10 minutes old and still Pending.
- Failed:  All Containers in the Pod have terminated, and at least one Container has terminated in failure. That is, the Container either exited with non-zero status or was terminated by the system.
- Unknown:  For some reason the state of the Pod could not be obtained, typically due to an error in communicating with the host of the Pod.

#### Options

By default, `Pod Status Check` will check pods in the same namespace it is installed into.  This means the RBAC requirements for the service account the check runs with can be limited to a single namespace scope.

It is possible to configure `Pod Status Check` to check pods from all namespaces in a cluster, this requires cluster wide permissions for the service account and is not recommended for multi-tenant setups.

#### How-to

##### kubectl apply
To implement the Pod Status Check with Kuberhealthy, apply the configuration file [pod-status-check.yaml](pod-status-check.yaml)

`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/2.0.0/cmd/pod-status-check/pod-status-check.yaml` to your Kubernetes Cluster.

If you want to enable the cluster wide option described above then __instead__ apply with cluster permissions [pod-status-check-clusterscope.yaml](pod-status-check-clusterscope.yaml).

##### Helm

```
helm repo add kuberhealthy https://comcast.github.io/kuberhealthy/helm-repos

helm install kuberhealthy kuberhealthy/kuberhealthy --set check.podStatus.enabled=true
```

To enable cluster wide check with cluster permissions
```
helm install kuberhealthy kuberhealthy/kuberhealthy --set check.podStatus.enabled=true --set check.podStatus.allNamespaces=true
```
