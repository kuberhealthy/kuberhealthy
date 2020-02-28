### External Checks Registry

Here is a list of external checks you can apply to your kubernetes cluster once you have Kuberhealthy installed.  For convenient addition directly from the web, ensure you have Kuberhealthy installed in your cluster and run `kubectl apply -f` on the `khcheck` resource URL of your choice below.  For easy cleanup, just run `kubectl delete -f` on the same URL.

| Check Name | Description | khcheck Resource | Contributor |
| --- | --- | --- | --- |
| [Daemonset Check](../cmd/daemonSetCheck/README.md) | Ensures daemonsets can be successfully deployed | [daemonSetCheck.yaml](../cmd/daemonSetCheck/daemonSetCheck.yaml) | @integrii @joshulyne |
| [Deployment Check](../cmd/deployment-check/README.md) | Ensures that a Deployment and Service can be provisioned, created, and serve traffic within the Kubernetes cluster | [deployment-check.yaml](../cmd/deployment-check/deployment-check.yaml) | @jonnydawg |
| [Pod Restarts Check](../cmd/podRestartsCheck/README.md) | Checks for excessive pod restarts in any namespace | [podRestartsCheck.yaml](../cmd/podRestartsCheck/podRestartsCheck.yaml) | @integrii @joshulyne |
| [Pod Status Check](../cmd/podStatusCheck/README.md) | Checks for unhealthy pod statuses in a target namespace | [podStatusCheck.yaml](../cmd/podStatusCheck/podStatusCheck.yaml) | @integrii @rukatm |
| [DNS Status Check](../cmd/dnsStatusCheck/README.md) | Checks for failures with DNS, including resolving within the cluster and outside of the cluster | [externalDNSStatusCheck.yaml](../cmd/dnsStatusCheck/externalDNSStatusCheck.yaml) [internalDNSStatusCheck.yaml](../cmd/dnsStatusCheck/internalDNSStatusCheck.yaml) | @integrii @joshulyne |
| [HTTP Check](../cmd/http-check/README.md)| Checks that a URL endpoint can serve a 200 OK response | [http-check.yaml](../cmd/http-check/http-check.yaml) | @jonnydawg |
| [KIAM Check](../cmd/kiam-check/README.md) | Checks that KIAM Servers and Agents are able to provide credentials | [kiam-check.yaml](../cmd/kiam-check.yaml) | @jonnydawg |

If you have a check you would like to share with the community, please open a PR to this file!
