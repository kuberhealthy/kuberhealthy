## Image Download Check
The `Image Download Check` pulls images from a specified registry set by env `FULL_IMAGE_URL` and evaluates if the actual pull duration time exceeded the configured timeout limit set by env `TIMEOUT_LIMIT`. If the pull duration exceeds the configured timeout, the check fails and is reported to Kuberhealthy servers. If the registry requires authentication, then set the `LOGIN_REQUIRED` environment variable to `true` and provide the username and password using `REGISTRY_USERNAME` and `REGISTRY_PASSWORD` environment variables.

#### Example Image Download Check Kubernetes Spec

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: image-download-check
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 25m
  podSpec:
    containers:
      - name: image-download-check
        image: kuberhealthy/image-download-check:v0.0.1
        imagePullPolicy: IfNotPresent
        env:
          - name: FULL_IMAGE_URL
            value: "nginx:1.21"   #### example: docker pull nginx:1.21
          - name: TIMEOUT_LIMIT
            value: "180s"         #### in seconds
          - name: LOGIN_REQUIRED
            value: "true"        #### set to true if the registry requires login
          - name: REGISTRY_USERNAME
            value: "username"    #### registry username
          - name: REGISTRY_PASSWORD
            value: "password"    #### registry password
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
