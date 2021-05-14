## SSL Expiration Status Check

The *SSL Expiry Check* checks that SSL certificates are not currently expired, and that the expiration date is a specified number of days away (60 by default).

If running more than one SSL expiry check, the metadata name field should be updated to avoid confusion and over-writing of checks.

The check runs daily (spec.runInterval), with a check timeout set to 15 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

The SSL Self-signed Expiry Check spec toggles *InsecureSkipVerify* to **true**. This bypasses the TLS handshake process and is only intended to be used with self-signed certificates.

#### SSL CA Expiry Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-expiry
  namespace: kuberhealthy
spec:
  runInterval: 24h
  timeout: 15m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "corporate.comcast.com"
          # If not using default SSL port of 443, port name env variable must be updated
          - name: PORT
            value: "443"
          # Number of days until certificate expiration to test for
          - name: DAYS
            value: "60"
          # Set INSECURE to "false" for CA issued certificates. If "false", a TLS handshake will be performed during the expiry check.
          - name: INSECURE
            value: "false"
        image: kuberhealthy/ssl-expiry-check:v3.2.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### SSL Self-signed Expiry Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-expiry
  namespace: kuberhealthy
spec:
  runInterval: 24h
  timeout: 15m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "kubernetes.default"
          # If not using default SSL port of 443, port name env variable must be updated
          - name: PORT
            value: "443"
          # Number of days until certificate expiration to test for
          - name: DAYS
            value: "60"
          # Set INSECURE to "true" for self-signed certificates. If "true", the TLS handshake will be skipped. This only checks expiration status, NOT validity/security.
          - name: INSECURE
            value: "true"
        image: kuberhealthy/ssl-expiry-check:v3.2.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi

```

#### How-to

To implement the SSL Handshake Check with Kuberhealthy, update the spec sheet with the domain name, port number, and number of days until expiration that you wish to test. TLS handshakes will fail on self-signed certificates. To check expiration dates and timeframes for self-signed certificates, skip that step by setting the INSECURE variable to "true".

#### Update values as needed and apply the spec sheets:

`kubectl apply -f ssl-ca-expiry-check.yaml`
or
`kubectl apply -f ssl-selfsign-expiry-check.yaml`


#### You can also use the default values by running:
`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/v2.3.0/cmd/ssl-expiry-check/ssl-ca-expiry-check.yaml`
or
`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/v2.3.0/cmd/ssl-expiry-check/ssl-selfsign-expiry-check.yaml`
