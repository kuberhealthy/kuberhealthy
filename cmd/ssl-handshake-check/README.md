## SSL Handshake Status Check

The *SSL Handshake Check* checks that SSL certificates are valid and a TLS handshake can be completed successfully against the domain/port specified.

The check runs every 5 minutes (spec.runInterval), with a check timeout set to 10 minutes (spec.timeout). If the check
does not complete within the given timeout it will report a timeout error on the status page.

#### NOTES:
Certificate Authority (CA) issued certificates should use the first spec sheet (ssl-handshake-check.yaml) with the SELF_SIGNED env var set to "false".

Self-signed certificate checks can be performed two different ways. The first spec below can be set to SELF_SIGNED = "true" and the check will attempt to retrieve the certificate from the host, add it to the cert pool, and run the TLS handshake check.

The second spec can be used if you have the certificate. Copy and paste the self-signed certificate data into the configmap spec under the certificate.crt data label. 

See the examples below: 

#### EXAMPLE SSL Handshake Check - Self Signed or CA Issued Kube Spec (ssl-handshake-check.yaml):
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-handshake
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "kubernetes.default"
          # If not using default SSL port of 443, port env variable must be updated  
          - name: PORT
            value: "443"
          # For internal, self-signed certs, set to "true" and the handshake check will attempt to automatically retrieve the host certificate
          - name: SELF_SIGNED
            value: "true"
        image: kuberhealthy/ssl-handshake-check:v3.1.11
        imagePullPolicy: IfNotPresent
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### EXAMPLE SSL Handshake Check - Self Signed, with Certificate in Spec (ssl-file-handshake-check.yaml):
```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ssl-handshake
  namespace: kuberhealthy
spec:
  runInterval: 5m
  timeout: 10m
  podSpec:
    containers:
      - env:
          # Domain name env variable must be updated to the domain on which you wish to check the SSL for
          - name: DOMAIN_NAME
            value: "kubernetes.default"
          # If not using default SSL port of 443, port env variable must be updated  
          - name: PORT
            value: "443"
          # For internal, self-signed certificates, set to "true" and copy and paste the .pem formatted certificate in the config map below 
          - name: SELF_SIGNED
            value: "true"
        image: kuberhealthy/ssl-handshake-check:v3.1.11
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
  # The certificate below "certificate.crt" MUST be replaced with your own self-signed SSL certificate.
  certificate.crt: |
    -----BEGIN CERTIFICATE-----
    MIIDrDCCApSgAwIBAgIIdW/eznBkz2swDQYJKoZIhvcNAQELBQAwFTETMBEGA1UE
    AxMKa3ViZXJuZXRlczAeFw0yMDAxMTUxNTI0NTlaFw0yMTAxMTQxNTI0NTlaMBkx
    FzAVBgNVBAMTDmt1YmUtYXBpc2VydmVyMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
    MIIBCgKCAQEArNLKINqzuNswX/5LMdNh4HLm87Xj13srm0c3al0T7oCvBLJVBU8o
    KF4jTlB3xE+LqZLJHU6lO/JNw+w76sAnFIK1n2NSKhw66O5gO0eyL2MIq5KCj2wW
    2FI5ZmshbHWa4OB/1+YSYWhB9WC/FLhIK4DTtKsiqkPB7GCnfZVFyEI0mavpbSRd
    7K0Vxs5E0QWp3+h4uRfMBW8/LqzI+Br8ZC1AxCeJT9IcJU1kaf0+MGFmxG2oPIRx
    z1sF8RFlqYeVTdhUnvRdNdmSnhPNhjeLIZOREJBRfUAd3xbp24U+FG97vv7g54vQ
    3CkL/aR2dbTTEmltdy8+TPPo04+JP7Y/XQIDAQABo4H7MIH4MA4GA1UdDwEB/wQE
    AwIFoDATBgNVHSUEDDAKBggrBgEFBQcDATCB0AYDVR0RBIHIMIHFghJkb2NrZXIt
    Zm9yLWRlc2t0b3CCCmt1YmVybmV0ZXOCEmt1YmVybmV0ZXMuZGVmYXVsdIIWa3Vi
    ZXJuZXRlcy5kZWZhdWx0LnN2Y4Ika3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVz
    dGVyLmxvY2FsghprdWJlcm5ldGVzLmRvY2tlci5pbnRlcm5hbIISdm0uZG9ja2Vy
    LmludGVybmFsgglsb2NhbGhvc3SHBApgAAGHBAAAAACHBMCoQQOHBH8AAAEwDQYJ
    KoZIhvcNAQELBQADggEBAKZtcxAioIx3XjgoVGkCA5TXu6derKkycltewNOz+LSV
    UsQzHABX0MQkGe6Mmi5GMgcTPtgIu+yfQJjWw+cEwd79cAnpX1HZ6uWAo1elhCKg
    IIpVQ3Xmc0CKAWAIfrEmetav1fMFx3qVcGqWLFy8/9eOYkjClNh0zCgX+V0Q4FqUFRx
    rzOpBihYH00htyNcfq8GzlKBO7vumxIkDDo5EgHxpU5LbKKyXaiN1+mdmcunojZA
    7VcIuL/NsOFIrFjP+9poMYkeRU3WRf+bsXCu8/qBB41QJbSO3sDq1PjusXe+iKMx
    GiELoUtIiPU6U/rU3M8o2EiDugD3hwr7oY7BWAUtaPg=
    -----END CERTIFICATE-----
```
#### How-to

To implement the SSL Handshake Check with Kuberhealthy, update the spec sheet to the domain name and port number you wish to test and apply:  

`kubectl apply -f ssl-handshake-check.yaml`  

You can use the default values as well by running:  

`kubectl apply -f https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/v2.3.0/cmd/ssl-handshake-check/ssl-handshake-check.yaml`  

To use the `ssl-file-handshake-check.yaml` spec sheet, you must first update the certificate configmap.

 Make sure you are using the latest release of Kuberhealthy 2.3.0.
