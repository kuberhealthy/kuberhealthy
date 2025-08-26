# Troubleshooting

If Kuberhealthy checks are failing:

- Ensure the operator pod is running:
  ```sh
  kubectl get pods -n kuberhealthy
  ```
- Inspect logs for errors:
  ```sh
  kubectl logs -n kuberhealthy deployment/kuberhealthy
  ```
- Review `khcheck` resources for detailed error messages:
  ```sh
  kubectl get khcheck -n kuberhealthy -o yaml
  ```
- Confirm the status page and metrics endpoint are reachable:
  ```sh
  kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80
  curl -f localhost:8080/metrics
  ```
