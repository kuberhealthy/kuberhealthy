## DNS Status Check

The *DNS Status Check* checks for failures with DNS, including resolving within the cluster and outside of the cluster.
Each check lookups and verifies one hostname. Default KHCheck configurations applied are:
- [internalDNSStatusCheck.yaml](internalDNSStatusCheck.yaml)
    - Hostname: kubernetes.default
    - Check Name: dns-status-internal
- [externalDNSStatusCheck.yaml](externalDNSStatusCheck.yaml)
    - Hostname: google.com
    - Check Name: dns-status-external

The check runs every 2 minutes (spec.runInterval), with a check timeout set to 15 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

To verify other hostnames, apply another KHCheck configuration file with a different `HOSTNAME` environment variable.

#### DNS Status Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: dns-status-internal
  namespace: kuberhealthy
spec:
  runInterval: 2m
  timeout: 15m
  podSpec:
    containers:
      - env:
          - name: HOSTNAME
            value: "kubernetes.default"
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        image: kuberhealthy/dns-resolution-check:v1.4.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the DNS Status Check with Kuberhealthy, run

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/2.0.0/cmd/dns-resolution-check/externalDNSStatusCheck.yaml`
`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/2.0.0/cmd/dns-resolution-check/internalDNSStatusCheck.yaml`

 Make sure you are using the latest release of Kuberhealthy 2.0.0.
