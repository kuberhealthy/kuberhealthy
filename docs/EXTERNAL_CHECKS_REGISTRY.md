### External Checks Registry

Here is a list of external checks you can apply to your kubernetes cluster once you have Kuberhealthy installed.  For convenient addition directly from the web, ensure you have Kuberhealthy in your cluster and run `kubectl apply -f` on the `khcheck` resource URL.  For easy cleanup, just run `kubectl delete -f` on the `khcheck` resource URL.

Make sure to add your check here:

| Check Name | Description | khcheck Resource | Contributor |
| --- | --- | --- | --- |
| [Daemonset Check](../cmd/daemonSetExternalCheck/README.md) | Ensures daemonsets can be successfully deployed | [daemonSetExternalCheck.yaml](../cmd/daemonSetExternalCheck/daemonSetExternalCheck.yaml) | @integrii @joshulyne |
| Deployment Test | Ensures that a Deployment and Service can be provisioned and created within the Kubernetes cluster | ???? | @jonnydawg |
| [Pod Restarts Check](../cmd/podRestartsExternalCheck/README.md) | Checks for excessive pod restarts in any namespace | [podRestartsExternalCheck.yaml](../cmd/podRestartsExternalCheck/podRestartsExternalCheck.yaml) | @integrii @joshulyne |
