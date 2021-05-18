## http-content-check

The `http-content-check` searches through the content body of a chosen URL and determines if a specified string is present. If the request succeeds and the string is present, the check reports a pass. Otherwise, the check reports failure. `http-content-check` will follow header redirects and uses the standard [http.Client](https://golang.org/pkg/net/http/) behavior by default.

you can specify the URL to check with the `TARGET_URL` environment variable in the `.yaml` file.
You can specify the string to look for with the `TARGET_STRING` environment variable in the `.yaml` file.

#### Example http-content-check Spec

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: http-content-check
spec:
  runInterval: 60s # The interval that Kuberhealthy will run your check on
  timeout: 2m # After this much time, Kuberhealthy will kill your check and consider it "failed"
  podSpec: # The exact pod spec that will run.  All normal pod spec is valid here.
    containers:
      - image: kuberhealthy/http-content-check:v1.5.0 # The image of the check you just pushed
        imagePullPolicy: IfNotPresent # uses local image if present
        name: main
        env:
          - name: "TARGET_URL"
            value: "http://httpbin.org" # The URL that application will use to look for a specified string
          - name: "TARGET_STRING"
            value: "httpbin" # The string that will be used to parse through provided URL
          - name: "TIMEOUT_DURATION"
            value: "30s" # Specifies the time limit for requests made by the client to the URL
```

#### How-to

Apply a `.yaml` file similar to the one shown above with `kubectl apply -f`
