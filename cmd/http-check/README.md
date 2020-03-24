## HTTP Get Request Check

The HTTP check sends a *GET* request to a specified URL, looking for a 200 OK response. This check reports a success upon receiving a 200 OK and a failure on any other status code.

You can specify the URL to check with the `CHECK_URL` environment variable in the `.yaml` file.

The check will exit without sending a request if the specified URL is not prefixed with an HTTP or HTTPS protocol.

#### Example HTTP Check Spec
```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: http
  namespace: kuberhealthy
spec:
  runInterval: 2m
  timeout: 6m
  podSpec:
    containers:
    - name: http
      image: kuberhealthy/http-check:v1.1.1
      imagePullPolicy: IfNotPresent
      env:
        - name: CHECK_URL
          value: "http://google.com"
      resources:
        requests:
          cpu: 15m
          memory: 15Mi
        limits:
          cpu: 25m
      restartPolicy: Always
    terminationGracePeriodSeconds: 5
```

#### How-to

 Make sure you are using the latest release of Kuberhealthy 2.0.0.

 Apply a `.yaml` file similar to the one shown above with ```kubectl apply -f```
