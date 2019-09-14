#### testExternalCheck

An example checker pod for testing the external check functionality.  Waits a few seconds and reports success every time.  Logs any Kuberhealthy API communication failures.

The spec for enabling this check in Kuberhealthy looks like this:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-test-check
  namespace: kuberhealthy
spec:
  runInterval: 30s
  podSpec:
    containers:
    - env:
      - name: SOME_ENV_VAR
        value: "12345"
      image: testexternalcheck:latest
      imagePullPolicy: IfNotPresent
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```

Apply this check in your cluster (normally for testing) by running `kubectl apply -f khcheck.yaml`.

This of course requires the new `khcheck` CRD to be configured.  The yaml for that looks like this:

```
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: khchecks.comcast.github.io
spec:
  group: comcast.github.io
  version: v1
  scope: Namespaced
  names:
    plural: khchecks
    singular: khcheck
    kind: KuberhealthyCheck
    shortNames:
      - khc
```
