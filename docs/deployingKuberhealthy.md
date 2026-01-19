# Deploying Kuberhealthy

Kuberhealthy ships Kustomize manifests in `deploy/kustomize` and a Helm chart in `deploy/helm/kuberhealthy`.

## Kustomize

```sh
kubectl apply -k deploy/kustomize/base
```
If you apply directly from GitHub, track the latest `main` commit until release tags are published:

```sh
kubectl apply -k github.com/kuberhealthy/kuberhealthy/deploy/kustomize/base?ref=main
```

The base Service is `kuberhealthy` on port `80` (forwarded to the pod on `8080`).

## Helm (v3 chart)

```sh
helm install kuberhealthy deploy/helm/kuberhealthy -n kuberhealthy --create-namespace
```
Pin to a release tag when chart packages are available.

The Service is `kuberhealthy` on port `80` (forwarded to the pod on `8080`).
The chart defaults `service.type` to `LoadBalancer`; set `service.type=ClusterIP` if you do not want an external load balancer.

## ArgoCD

```sh
kubectl apply -f deploy/argocd/kuberhealthy.yaml
```

## RBAC requirements

Kuberhealthy installs a ClusterRole and ClusterRoleBinding for the controller ServiceAccount. The default permissions are defined in `deploy/kustomize/base/clusterrole.yaml` and include:

- `kuberhealthy.github.io` `healthchecks` resources (full CRUD + status/finalizers).
- Core `pods`, `pods/log`, and `events`.
- `coordination.k8s.io` `leases` (needed for leader election).

If you need to scope to a namespace, create a dedicated Role/RoleBinding and confirm the controller can still create and read checker pods in the target namespaces.

## Scaling and leader election

Kuberhealthy can run multiple controller replicas with leader election enabled. Only the leader runs checks and reaps pods, while all replicas serve the UI and APIs.

- Set `KH_LEADER_ELECTION_ENABLED=true` and ensure the controller service account can read/write `coordination.k8s.io` `leases`.
- If you run more than one replica, verify your affinity/tolerations allow scheduling on available nodes. The Helm chart uses pod anti-affinity to spread replicas, which can block scheduling in single-node clusters unless overridden.

## Expose the status page

The `/json` endpoint is served from the Service. Use a cloud-specific overlay or an ingress:

```sh
# AWS
kubectl apply -k deploy/kustomize/aws-lb-controller

# GCP
kubectl apply -k deploy/kustomize/gcp-lb-controller

# Azure
kubectl apply -k deploy/kustomize/azure-lb-controller

# Ingress (on-prem or controller-managed)
kubectl apply -k deploy/kustomize/ingress
```

## TLS and HTTPS

Kuberhealthy can serve HTTPS when both `KH_TLS_CERT_FILE` and `KH_TLS_KEY_FILE` are set. The HTTPS listener uses `KH_LISTEN_ADDRESS_TLS` (default `:443`) while the HTTP listener continues to use `KH_LISTEN_ADDRESS`.

At minimum you must:

- Mount a TLS secret into the controller pod.
- Set `KH_TLS_CERT_FILE` and `KH_TLS_KEY_FILE` to the mounted paths.
- Expose or route to port `443` on the pod (Service/Ingress update).

If you are using Kustomize, create an overlay that patches the deployment and Service. Example patch snippets:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kuberhealthy
spec:
  template:
    spec:
      containers:
        - name: kuberhealthy
          env:
            - name: KH_TLS_CERT_FILE
              value: /etc/kuberhealthy/tls/tls.crt
            - name: KH_TLS_KEY_FILE
              value: /etc/kuberhealthy/tls/tls.key
          volumeMounts:
            - name: kuberhealthy-tls
              mountPath: /etc/kuberhealthy/tls
              readOnly: true
      volumes:
        - name: kuberhealthy-tls
          secret:
            secretName: kuberhealthy-tls
```

```yaml
apiVersion: v1
kind: Service
metadata:
  name: kuberhealthy
spec:
  ports:
    - name: https
      port: 443
      targetPort: 443
```

## Configure

All configuration is via environment variables in the deployment. See [FLAGS.MD](FLAGS.MD).

## Verify

```sh
kubectl -n kuberhealthy get pods
kubectl -n kuberhealthy port-forward svc/kuberhealthy 8080:80 &
curl -fsS localhost:8080/metrics
```
