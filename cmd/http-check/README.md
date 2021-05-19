## HTTP Get Request Check

The HTTP check can be configured to send a _GET_ / _POST_ / _PUT_ / _DELETE_ / _PATCH_ request to a specified URL, looking for a certain specific status code's response. This check reports a success upon receiving a response with the status code matching with the expected status code (like 200 or 204, etc.) and a failure on any other status code.

You can specify the URL to check with the `CHECK_URL` environment variable in the `.yaml` file.

The check will exit without sending a request if the specified URL is not prefixed with an HTTP or HTTPS protocol.

### Example HTTP Check Specs

**GET check**

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: http
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - name: https
        image: kuberhealthy/http-check:v1.5.0
        imagePullPolicy: IfNotPresent
        env:
          - name: CHECK_URL
            value: "https://reqres.in/api/users"
          - name: COUNT #### default: "0"
            value: "5"
          - name: SECONDS #### default: "0"
            value: "1"
          - name: PASSING_PERCENT #### default: "100"
            value: "80"
        resources:
          requests:
            cpu: 15m
            memory: 15Mi
          limits:
            cpu: 25m
    restartPolicy: Always
    terminationGracePeriodSeconds: 5
```

**POST check**

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: http
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - name: https
        image: kuberhealthy/http-check:v1.5.0
        imagePullPolicy: IfNotPresent
        env:
          - name: CHECK_URL
            value: "https://reqres.in/api/users"
          - name: COUNT #### default: "0"
            value: "1"
          - name: SECONDS #### default: "0"
            value: "1"
          - name: PASSING_PERCENT #### default: "100"
            value: "100"
          - name: REQUEST_TYPE #### default: "GET"
            value: "POST"
          - name: REQUEST_BODY #### default: "{}"
            value: '{"name": "morpheus", "job": "leader"}'
          - name: EXPECTED_STATUS_CODE #### default: "200"
            value: "201"
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

Apply a `.yaml` file similar to the one shown above with `kubectl apply -f`
