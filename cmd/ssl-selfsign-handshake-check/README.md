## SSL Self-signed Handshake Status Check

The *SSL Self-signed Handshake Check* checks that self-signed SSL certificates are valid and a TLS handshake can be completed successfully against the domain/port specified.

The check runs every 5 minutes (spec.runInterval), with a check timeout set to 3 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

You must copy and paste the self-signed certificate data into the configmap spec under the certificate.crt data label. See the example below:

#### SSL Handshake Check Kube Spec:
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-selfsign-handshake
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 3m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "localhost"
          # If not using default SSL port of 443, port env variable must be updated  
          - name: PORT
            value: "443"
        image: kuberhealthy/ssl-selfsign-handshake-check:v1.2.0
        imagePullPolicy: IfNotPresent
        name: main
        volumeMounts:
        - name: certificate-vol
          mountPath: /etc/ssl/selfsign  
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
    volumes:
      - name: certificate-vol
        configMap:
          name: certificate-config
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: certificate-config
  namespace: kuberhealthy
data:
  certificate.crt: |
    -----BEGIN CERTIFICATE-----
    MIIFzjCCA7YCCQCKlhTYSY1jmDANBgkqhkiG9w0BAQsFADCBqDELMAkGA1UEBhMC
    VVMxFTATBgNVBAgMDFBlbm5zeWx2YW5pYTEVMBMGA1UEBwwMUGhpbGFkZWxwaGlh
    MRAwDgYDVQQKDAdDb21jYXN0MRowGAYDVQQLDBFDbG91ZCBFbmdpbmVlcmluZzES
    MBAGA1UEAwwJbG9jYWxob3N0MSkwJwYJKoZIhvcNAQkBFhp6YWNoYXJ5X2hhbnNv
    bkBjb21jYXN0LmNvbTAeFw0yMDA4MjEwMTU3NDFaFw0yMTA4MjEwMTU3NDFaMIGo
    MQswCQYDVQQGEwJVUzEVMBMGA1UECAwMUGVubnN5bHZhbmlhMRUwEwYDVQQHDAxQ
    aGlsYWRlbHBoaWExEDAOBgNVBAoMB0NvbWNhc3QxGjAYBgNVBAsMEUNsb3VkIEVu
    Z2luZWVyaW5nMRIwEAYDVQQDDAlsb2NhbGhvc3QxKTAnBgkqhkiG9w0BCQEWGnph
    Y2hhcnlfaGFuc29uQGNvbWNhc3QuY29tMIICIjANBgkqhkiG9w0BAQEFAAOCAg8A
    MIICCgKCAgEA4N8A/D+8sCESwAId6uj2n2nHIoQsFmTXdNfx7vKqxciPCy6Vq5nN
    THIS2CT0TTKOjFnmGDQka1QlBLLpG4RjkxWaOcF3d/SpKr5IJdePEgatoqw6vHS+
    6CWWPqA5gCHWEoyr8ObuaEWy//RBLBs2x0vIqpBXza+hJDPkOEKoRZPLZmyyK1eN
    pSzkYFgt/lycnWkcm7hamNwcoO9Xt53DSV+NnU7bZ/7gDEX9VI7IqAy1hgpLnTUl
    bfwqtXYehjlswgJGaCv9LHa/hOUQVDdnVPLUILXLFfmC5dEFS+/zBO1FCzVz/i9K
    lHWJmSZ6L7FHzyAsx5rGEcF9YYjr+Ne5D83gC86XQnliCollDR+olAgLLCh0aF6O
    dBE5w5UOrlwZKOCQDhauOaV4w7UTqEKPv/WdCxmCIrLyujLoFwTLEQXEH9XAX8OO
    a+Hk3bv3jeC0WiCWquhzqh89x4dr2fJhmwexv4BCNvUD8WPZXPwXbBG34Hz54cTg
    /I62xhcp0qWw03olTbi0SPaVJUksOZ301tCNdN8xUG6DyKV6aZ3sJKT3vs46zvAz
    BVN+741UcQCLebtqIlndxHN7Biw9+nbhGBZObyL0iJeRcDaOGcOtuU/qj9ntvrsn
    ro2N8/ZOM7uDeNt3Jrzk1LYg1eS4whEw8NnHvuPg/HIrPJUAascFfr8CAwEAATAN
    BgkqhkiG9w0BAQsFAAOCAgEAA2WAAinFm/0lcznUbmrAlfMkgw6x8Ju6woZLXP67
    65zwj6MwWMl3oKKVTINv9qaT/tXtWlxVkSGHV07024IdmiaN5y6k/uyj1cHBbDDj
    gaccEMHrJofwxS1rUkS/OqM0veSkN7KtLrs2JJIA9JV8B0kJx1AyweT793zsd4PZ
    rjDMrEj4euJaL+K3G8cnMwH0T43NJQxdZ1pFCapa0zN8DKuzQBnqP2Kydn4Yu8HF
    O2ghdlom1rdx0T3njhT00p5eYOkRLNM81yii4amufwgJjFXhu9tGC1bQAqamFA8/
    eoLeEbuThio1NQ3WcWZVGGqDJ8d0rb91R08bt2Uj5ZmLm3MdbHaZJF5ACTgEo/Z8
    L4xUYsxjVFj6q/QxqQ8yknpgJmtRkTAmwCJrwzgi3Gr47CEhZwHjx/BcHS5SXA8T
    vYJXg5iBHvMnuRbfK0/+6iXdw4f/fsEpixi+w9TwvRgQGPsrBYHL+Ci3Hak6/+fA
    DWXjT4yqkDhHvEXiQ/GV9nrEffE2c2ClW9s905mwPT7Ey09r4uzSWSu5ZtAOp3V1
    ebvx36F3DOePQrZiLqei06YZjVFmEPlI1/DX0r0Xyw4JdMxxBzzYQOz/DuvfwxZ6
    V4PGKNTve6HVj3Ar/aHL6DjvlPT5fyy970ZVbwXupHa2XJucCB+g8mMj/l1ubM4p
    uio=
    -----END CERTIFICATE-----


```

#### How-to

To implement the SSL Handshake Check with Kuberhealthy, update the spec sheet with the domain name, port number, and self-signed certificate you wish to test, and apply:

`kubectl apply -f ssl-selfsign-handshake-check.yaml`

You can use the default values as well by running:

`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/v2.3.0/cmd/ssl-selfsign-handshake-check/ssl-selfsign-handshake-check.yaml`

 Make sure you are using the latest release of Kuberhealthy 2.3.0.
