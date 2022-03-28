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
        image: kuberhealthy/dns-resolution-check:v1.5.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

### Checking endpoints behind DNS Service

In order to check endpoints behind the DNS service, add a `DNS_POD_SELECTOR` and `NAMESPACE` variable to the spec file denoting where your DNS pods are.

`DNS_POD_SELECTOR` is a label selector which will be used to select the DNS endpoints to query against.

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
          - name: NAMESPACE
            value: "kube-system"
          - name: DNS_POD_SELECTOR
            value: apps=k8s-dns
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        image: kuberhealthy/dns-resolution-check:v1.5.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the DNS Status Check with Kuberhealthy, run

`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/2.0.0/cmd/dns-resolution-check/externalDNSStatusCheck.yaml`
`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/2.0.0/cmd/dns-resolution-check/internalDNSStatusCheck.yaml`
