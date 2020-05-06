### test-external-check

This is a container used to test external checks in Kuberhealthy. This container simply runs, waits for a set interval of seconds and then either reports a success or failure depending on the `REPORT_FAILURE` environment variable set in the pod spec.  Simply set `REPORT_FAILURE` to `"true"` (a string not a bool) to cause this check to report an error message.


##### Example `khcheck`

The pod is designed to wait every few seconds before reporting a success or failure to the Kuberhealthy API. The pod is also design to log any errors it encounters when communicating to the Kuberhealthy API.

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
      image: kuberhealthy/test-external-check:v1.1.0
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```

You don't have to use the default settings in the above YAML. Change specs as needed to extend or shorten timeouts or run intervals.

##### Local Image Pull (optional)

By setting your external check pod spec `imagePullPolicy` to `Never` you will allow Kubernetes to pull the image locally from your Kubernetes hosts. This enables you to build the container inside of Kubernetes more rapidly instead of fetching it from the web.

See below on a example on how to set `imagePullPolicy`:

```yaml
    containers:
    - env:
      - name: REPORT_FAILURE
        value: "false"
      image: kuberhealthy/test-external-check:v1.1.0
      imagePullPolicy: Never
      name: main
```

### image-pull-check

This is a container used to test availability of external image respositories in Kuberhealthy.  Simply place the `kuberhealthy/test-external-check` image on the repository you need tested.

##### Example `khcheck`

The pod is designed to attempt a pull of the test image from the remote repository (never from local) every 10 minutes. If the image is unavavailable an error will be reported to the Kuberheakthy API.

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
      # test-external-check image must be uploaded to the repository you wish to test on, and below URL must be updated to match.
      image: kuberhealthy/test-external-check:v1.1.0
      imagePullPolicy: Always
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
  runInterval: 10m
  timeout: 1m
```
You can change the default specs as neededin the above YAML to extend or shorten timeouts, run intervals, etc.
