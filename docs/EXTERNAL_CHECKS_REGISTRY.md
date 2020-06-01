### External Checks Registry

Here is a list of external checks you can apply to your kubernetes cluster once you have Kuberhealthy installed.  For convenient addition directly from the web, ensure you have Kuberhealthy installed in your cluster and run `kubectl apply -f` on the `khcheck` resource URL of your choice below.  For easy cleanup, just run `kubectl delete -f` on the same URL.

| Check Name | Description | khcheck Resource | Contributor |
| --- | --- | --- | --- |
| [Daemonset Check](../cmd/daemonset-check/README.md) | Ensures daemonsets can be successfully deployed | [daemonset-check.yaml](../cmd/daemonset-check/daemonset-check.yaml) | @integrii @joshulyne |
| [Deployment Check](../cmd/deployment-check/README.md) | Ensures that a Deployment and Service can be provisioned, created, and serve traffic within the Kubernetes cluster | [deployment-check.yaml](../cmd/deployment-check/deployment-check.yaml) | @jonnydawg |
| [Pod Restarts Check](../cmd/pod-restarts-check/README.md) | Checks for excessive pod restarts in any namespace | [pod-restarts-check.yaml](../cmd/pod-restarts-check/pod-restarts-check.yaml) | @integrii @joshulyne |
| [Pod Status Check](../cmd/pod-status-check/README.md) | Checks for unhealthy pod statuses in a target namespace | [pod-status-check.yaml](../cmd/pod-status-check/pod-status-check.yaml) | @integrii @rukatm |
| [DNS Status Check](../cmd/dns-resolution-check/README.md) | Checks for failures with DNS, including resolving within the cluster and outside of the cluster | [externalDNSStatusCheck.yaml](../cmd/dns-resolution-check/externalDNSStatusCheck.yaml) [internalDNSStatusCheck.yaml](../cmd/dns-resolution-check/internalDNSStatusCheck.yaml) | @integrii @joshulyne |
| [HTTP Check](../cmd/http-check/README.md)| Checks that a URL endpoint can serve a 200 OK response | [http-check.yaml](../cmd/http-check/http-check.yaml) | @jonnydawg |
| [KIAM Check](../cmd/kiam-check/README.md) | Checks that KIAM Servers and Agents are able to provide credentials | [kiam-check.yaml](../cmd/kiam-check/kiam-check.yaml) | @jonnydawg |
| [HTTP Content Check](../cmd/http-content-check/README.md) | Checks for specific string in body of URL | [http-content-check.yaml](../cmd/http-content-check/http-content-check.yaml) | @jdowni000 |
| [Resource Quota Check](../cmd/resource-quota-check/README.md) | Checks if resource quotas (CPU & memory) are available | [resource-quota.yaml](../cmd/resource-quota-check/resource-quota.yaml) | @jonnydawg |
| [Network Connection Check](../cmd/network-connection-check/README.md) | Checks if a network connection (tcp or udp) could be done to a remote target | [successedNetworkConnectionCheck.yaml](../cmd/successedNetworkConnectionCheck.yaml) [failedNetworkConnectionCheck.yaml](../cmd/network-connection-check/failedNetworkConnectionCheck.yaml) | @bavarianbidi |

If you have a check you would like to share with the community, please open a PR to this file!
