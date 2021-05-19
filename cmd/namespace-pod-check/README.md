## Namespace Pod Checker

This container retrieves a list of all namespaces and will attempt to deploy pods in all namespaces.
Once successful, it will delete the pod.
100% succesful deployment and deletion of test podsin all namespaces will report success to kuberhealthy.

#### Example Namespace Pod Checker Spec

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: namespace-pod-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - name: namespace-pod-check
        image: kuberhealthy/namespace-pod-check:v1.1.0
        imagePullPolicy: IfNotPresent
        resources:
          requests:
            cpu: 15m
            memory: 15Mi
          limits:
            cpu: 25m
    restartPolicy: Always
    terminationGracePeriodSeconds: 5
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: namespace-pod-check
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kuberhealthy
subjects:
  - kind: ServiceAccount
    name: namespace-pod-check
    namespace: kuberhealthy
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: namespace-pod-check
  namespace: kuberhealthy
```

#### How-to

Apply a `.yaml` file similar to the one shown above with `kubectl apply -f`
