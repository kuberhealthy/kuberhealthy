## SSL Handshake Status Check

The *SSL Handshake Check* checks that SSL certificates are valid and a TLS handshake can be completed successfully against the domain/port specified.

The check runs every 5 minutes (spec.runInterval), with a check timeout set to 3 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

#### NOTE:
Self-signed certificate checks are not supported by this check. Please use the *ssl-selfsign-handshake* check for TLS handshake checks on self-signed certificates.

#### SSL Handshake Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-handshake
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 3m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "google.com"
          # If not using default SSL port of 443, port name env variable must be updated  
          - name: PORT
            value: "443"
        image: kuberhealthy/ssl-handshake-check:v1.2.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the SSL Handshake Check with Kuberhealthy, update the spec sheet to the domain name and port number you wish to test and apply:  

`kubectl apply -f ssl-handshake-check.yaml`  

You can use the default values as well by running:  

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/v2.3.0/cmd/ssl-handshake-check/ssl-handshake-check.yaml`  

 Make sure you are using the latest release of Kuberhealthy 2.3.0.
