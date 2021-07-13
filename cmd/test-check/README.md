### test-check

This is a container used to test check functionality in Kuberhealthy. This container simply runs, waits for a set interval of seconds and then either reports a success or failure depending on the `REPORT_FAILURE` environment variable set in the pod spec.


##### Environment Variables

`REPORT_FAILURE` can be set to `"true"` to cause the check to report a failure instead of success.
`REPORT_DELAY` can be set to a duration of time (5s or 3h for example) to configure how long the check waits before reporting back to Kuberhealthy.  By default this is set to 5s.


##### Example `khcheck`

The pod is also design to log any errors it encounters when communicating to the Kuberhealthy API.

Below is a YAML template for enabling the test-check in Kuberhealthy.

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
      image: kuberhealthy/test-check:v1.4.0
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```

### image-pull-check

This container tests the availability of image respositories using Kuberhealthy.  Simply `docker push` the `kuberhealthy/test-check` image on the repository you need tested.

The pod is designed to attempt a pull of the test image from the remote repository (never from local) every 10 minutes. If the image is unavailable, an error will be reported to the Kuberheakthy API.

To put a copy of this image to your repository, run `docker pull kuberhealthy/test-check` and then `docker push my.repository/repo/test-check`.


##### Example `khcheck`

Below is a YAML template for enabling the image-pull-check in Kuberhealthy.

Simply copy and paste the YAML specs below into a new file and apply it by using the command `kubectl apply -f your-named-khcheck.yaml`.

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: image-pull-check
  namespace: default
spec:
  podSpec:
    containers:
    - env:
      - name: REPORT_FAILURE
        value: "false"
      - name: REPORT_DELAY
        value: "1s"
      # test-check image must be uploaded to the repository you wish to test on, and below URL must be updated to match.
      image: kuberhealthy/test-check:v1.4.0
      imagePullPolicy: Always
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
  runInterval: 10m
  timeout: 1m
```

You can change the default specs as needed in the above YAML to extend or shorten timeouts, run intervals, etc.
