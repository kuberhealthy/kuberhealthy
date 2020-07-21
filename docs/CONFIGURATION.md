## Kuberhealthy Configmap 

Kuberhealthy uses a [configmap](https://kubernetes.io/docs/concepts/configuration/configmap/) for configuration parameters.  This configmap is monitored for changes by Kuberhealthy.  Upon a settings change being seen, all checks will be gracefully stopped and reloaded.  For check-specific configuration, options are stored in the relevant `khcheck` resource (`kubectl get khchecks`).

The configuration file is mounted at `/etc/config'


#### Example Configmap

The following configmap contains all configurable options.  If a configuraiton parameter is not needed, simply leave it blank or remove it from the configmap.

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: kuberhealthy
data:
  kuberhealthy.yaml: |-
    listenAddress: ":8080" # The port for kuberhealthy to listen on for web requests
    enableForceMaster: false # Set to true to enable local testing, forced master mode
    logLevel: "debug" # Log level to be used
    influxUsername: "" # Username for the InfluxDB instance
    influxPassword: "" # Password for the InfluxDB instance
    influxURL: "" # Address for the InfluxDB instance
    influxDB: "http://localhost:8086" # Name of the InfluxDB database
    enableInflux: false # Set to true to enable metric forwarding to Infux DB
```
