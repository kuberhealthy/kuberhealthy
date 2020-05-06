## Network Connection Check

The *Network Connection Check* checks for successful/unsuccessful connections to network targets (via `tcp` or `udp`).
Each check connect to one network target. Default KHCheck configurations applied are:
- [successedNetworkConnectionCheck.yaml](successedNetworkConnectionCheck.yaml)
    - Target: tcp://github.com:443
    - Target should be reachable: false
    - Check Name: kuberhealthy-github-reachable
- [failedNetworkConnectionCheck.yaml](failedNetworkConnectionCheck.yaml)
    - Hostname: tcp://169.254.169.254:80
    - Target should be unreachable: true
    - Check Name: kuberhealthy-metadata-unreachable

The check runs every 30 minutes (spec.runInterval), with a check timeout set to 125 seconds (spec.timeout). 
The spec.timeout value is also used inside the network connection check to define the timeout for the performed network connection (minus 5 seconds).

```
d := net.Dialer{LocalAddr: localAddr, Timeout: time.Duration(checkTimeout - (5000 * time.Millisecond))}
```

If the check
does not complete within the given timeout and target should be reachable it will report a timeout error on the status page.
If the check does not complete within the given timeout and target should not be reachable it will report a success on the status page.

To verify connections, apply another KHCheck configuration file with a different `CONNECTION_TARGET` environment variable.
`CONNECTION_TARGET` accepts `tcp://` or `udp://` (e.g. `udp://10.16.12.10:9000` or `tcp://10.16.12.11:8080`)

#### Network Connection Check Kube Spec:
```
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kuberhealthy-github-reachable
  namespace: kuberhealthy
spec:
  runInterval: 30m
  timeout: 125s
  podSpec:
    containers:
      - env:
          - name: CONNECTION_TARGET
            value: "tcp://github.com:443"
        image: kuberhealthy/network-connection-check:v0.1.0
        name: kuberhealthy-github-reachable
```

