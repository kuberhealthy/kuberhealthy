### test-external-check

This is a container used to test external checks in Kuberhealthy. This container simply runs, waits for a set interval of seconds and then either reports a success or failure depending on the `REPORT_FAILURE` environment variable set in the pod spec.  


##### Environment Variables

`REPORT_FAILURE` can be set to `"true"` to cause the check to report a failure instead of success.
`REPORT_DELAY` can be set to a duration of time (5s or 3h for example) to configure how long the check waits before reporting back to Kuberhealthy.  By default this is set to 5s.


##### Example `khcheck`

The pod is also design to log any errors it encounters when communicating to the Kuberhealthy API.

Below is a YAML template for enabling the test-external-check in Kuberhealthy.

Simply copy and paste the YAML specs below into a new file and apply it by using the command `kubectl apply -f your-named-khcheck.yaml`.


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
      - name: REPORT_DELAY
        value: "10s"
      image: kuberhealthy/test-external-check:v1.1.0
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```
