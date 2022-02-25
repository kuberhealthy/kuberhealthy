### Prometheus Integration

Kuberhealthy serves Prometheus metrics at the `/metrics` endpoint.  If you are using the [helm chart](https://github.com/kuberhealthy/kuberhealthy/tree/master/deploy/helm) to deploy Kuberhealthy, 
 enable Prometheus in the chart options to have scrape configs and annotations generated for you.

```bash
  helm install kuberhealthy kuberhealthy/kuberhealthy --set prometheus.enabled=true  --set prometheus.prometheusRule.enabled=true
```

Alternatively, you can use the static files that are generated from the helm chart auotmatically whenever the chart changes [here](https://github.com/kuberhealthy/kuberhealthy/blob/master/deploy/kuberhealthy-prometheus.yaml).
