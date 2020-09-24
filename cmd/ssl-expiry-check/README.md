## SSL Expiration Status Check

The *SSL Expiry Check* checks that SSL certificates are not currently expired, and that the expiration date is a specified number of days away (60 by default).

The check runs daily (spec.runInterval), with a check timeout set to 5 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

The SSL Self-signed Expiry Check spec toggles *InsecureSkipVerify* to **true**. This bypasses the TLS handshake process and is only intended to be used with self-signed certificates.

#### SSL Secure Expiry Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-secure-expiry
  namespace: kuberhealthy
spec:
  runInterval: 1d
  timeout: 3m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "corporate.comcast.net"
          # If not using default SSL port of 443, port name env variable must be updated  
          - name: PORT
            value: "443"
          - name: DAYS
            value: "60"
          - name: INSECURE
            value: "false"
        image: kuberhealthy/ssl-expiry-check:v1.2.0
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
  name: ssl-selfsign-expiry
  namespace: kuberhealthy
spec:
  runInterval: 1d
  timeout: 3m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "kubernetes.default"
          # If not using default SSL port of 443, port name env variable must be updated  
          - name: PORT
            value: "443"
          - name: DAYS
            value: "60"
          - name: INSECURE
            value: "true"
        image: kuberhealthy/ssl-expiry-check:v1.2.0
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to

To implement the SSL Handshake Check with Kuberhealthy, update the spec sheet to the domain name, port number, and number of days until expiration that you wish to test. TLS handshakes will fail on self-signed certificates. To check expiration dates and timeframes for self-signed certificates, skip that step by setting the INSECURE variable to "true". 

#### Update values as needed and apply the spec sheets:

`kubectl apply -f ssl-secure-expiry-check.yaml`  
or  
`kubectl apply -f ssl-selfsign-expiry-check.yaml`  


#### You can also use the default values by running:  
`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/v2.3.0/cmd/ssl-expiry-check/ssl-secure-expiry-check.yaml`  
or  
`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/v2.3.0/cmd/ssl-expiry-check/ssl-selfsign-expiry-check.yaml`  
 
 Make sure you are using the latest release of Kuberhealthy 2.3.0.