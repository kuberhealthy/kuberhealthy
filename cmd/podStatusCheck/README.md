## Pod Status Check

The *Pod Status Check* checks for pods older than ten minutes in the desired namespace that are in an incorrect 
lifecycle phase (anything that is not 'Ready' or 'Succeeded').  If a `podStatus` detects a pod down, an alert is shown 
on the status page. When a pod is found to be in error, the exact pod's name will be shown as one of the `Error` 
field's strings.  Set `metadata.namespace` to the intended namespace otherwise this check defaults to the `kube-system`.

#### Example Pod Status KuberhealtyCheck Spec
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-status-check
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 15m
  podSpec:
    containers:
      - env:
          - name: TARGET_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
        image: quay.io/comcast/pod-status-check:1.0.0
        imagePullPolicy: Always
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```



#### How-to

To implement the Pod Status Check with Kuberhealthy, apply the configuration file [podStatusCheck.yaml](podStatusCheck.yaml)
to your Kubernetes Cluster.  Make sure that you are using the latest release of Kuberhealthy 2.0.0.

To implement in a single namespace

To implement in multiple namespace

To implement in ALL namespaces


