## DNS Status Check

The *DNS Status Check* checks for failures with DNS, including resolving within the cluster and outside of the cluster. 
Each check lookups and verifies one hostname. Default KHCheck configurations applied are:
- [internalDNSStatusCheck.yaml](internalDNSStatusCheck.yaml)
    - Hostname: kubernetes.default
    - Check Name: dns-status-check-internal
- [externalDNSStatusCheck.yaml](externalDNSStatusCheck.yaml)
    - Hostname: google.com
    - Check Name: dns-status-check-external

The check runs every minute (spec.runInterval), with a check timeout set to 55 seconds (spec.timeout). If the check 
does not complete within the given timeout it will report a timeout error on the status page. The check takes in a 
`CHECK_POD_TIMEOUT` environment variable that ensures the the pod runs the hostname lookup within the timeout. 

To verify other hostnames, apply another KHCheck configuration file with a different `HOSTNAME` environment variable. 

#### Daemonset Check Kube Spec:
```
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: dns-status-check-internal
  namespace: kuberhealthy
spec:
  runInterval: 1m
  timeout: 55s
  podSpec:
    containers:
      - env:
          - name: CHECK_POD_TIMEOUT
            value: "45s"
          - name: HOSTNAME
            value: "kubernetes.default"
        image: quay.io/comcast/dns-status-check:1.0.0
        imagePullPolicy: Always
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the DNS Status Check with Kuberhealthy, apply both configuration files [internalDNSStatusCheck.yaml](internalDNSStatusCheck.yaml), 
[externalDNSStatusCheck.yaml](externalDNSStatusCheck.yaml), to your Kubernetes Cluster. Make sure you are using the latest release of Kuberhealthy 2.0.0. 