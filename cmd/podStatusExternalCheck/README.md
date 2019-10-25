### Pod Status Check

Checks for pods older than ten minutes in the `kube-system` namespace that are in an incorrect lifecycle phase (anything that is not 'Ready').  If a `podStatus` detects a pod down for 5 minutes, an alert is shown on the status page. When a pod is found to be in error, the exact pod's name will be shown as one of the `Error` field's strings.

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-status-check
  namespace: kuberhealthy
spec:
  runInterval: 2m
  timeout: 1m
  podSpec:
    containers:
      - env:
          - name: REPORT_FAILURE
            value: "false"
          - name: REPORT_DELAY
            value: "5s"
        image: quay.io/comcast/pod-status-check:1.0.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```













OLD DOCS

A command line flag exists `--podCheckNamespaces` which can optionally contain a comma-separated list of namespaces on which to run the podStatus checks.  The default value is `kube-system`.  Each namespace for which the check is configured will require the `get` and `list` [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) verbs on the `pods` resource within that namespace.

- Namespace: kube-system
- Timeout: 1 minutes
- Check Interval: 2 minutes
- Error state toleration: 5 minutes
- Check name: `podStatus`
