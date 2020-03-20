### test-external-check

This is a container for use in testing external checks on Kuberhealthy.  This container simply runs and either reports a pass or fail depending on the `REPORT_FAILURE` environment variable in its spec.

##### 2.0.0 alpha setup

The version 2.0.0 alpha for Kubernetes image is: `quay.io/comcast/kuberhealthy:2.0.0alpha`.  

You will need to run this image of kuberhealthy in your cluster before external checks are available to you.

You will also need to define the following CRD for Kuberhealthy 2 to use.

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


##### Example Check

An example checker pod for testing the external check functionality.  Waits a few seconds and reports success every time.  Logs any Kuberhealthy API communication failures.

The spec for enabling this check in Kuberhealthy looks like this:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-test-check
  namespace: kuberhealthy
spec:
  timeout: 2m
  extraAnnotations:
    comcast.com/testAnnotation: test.annotation
  extraLabels:
    testLabel: testLabel
  runInterval: 30s
  podSpec:
    containers:
    - env:
      - name: REPORT_FAILURE
        value: "false"
      image: kuberhealthy/test-external-check:v1.1.0
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```

Apply this check in your cluster (normally for testing) by running `kubectl apply -f khcheck.yaml`.

**Pro tip: by setting the `imagePullPolicy` to `Never` on your check's spec or on the kuberhealthy deoplyment in your test environment, the Docker Kubernetes cluster will pull the image from your local docker host instead of the web.  This enables rapid development when doing local builds.**
