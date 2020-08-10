## SSL Expiration Status Check

The *SSL Expiry Check* checks that SSL certificates are not currently expired, and that the expiration date is a specified number of days away (60 by default).

The check runs daily (spec.runInterval), with a check timeout set to 5 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

#### SSL Expiry Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-expiry-check
  namespace: kuberhealthy
spec:
  runInterval: 1d
  timeout: 5m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAINNAME
            value: "google.com"
          # If not using default SSL port of 443, port name env variable must be updated  
          - name: PORT
            value: "443"
          # Number of days until certificate expiration to test for  
          - name: DAYS
            value: "60"
        image: kuberhealthy/ssl-expiry-check:v1.0.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the SSL Expiry Check with Kuberhealthy, update the spec sheet to the domain name and port number you wish to test and, if necessary, the number of days until expiration you want to check against.

Run:
`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/v2.3.0/cmd/ssl-expiry-check/ssl-expiry-check.yaml`

 Make sure you are using the latest release of Kuberhealthy 2.3.0.
