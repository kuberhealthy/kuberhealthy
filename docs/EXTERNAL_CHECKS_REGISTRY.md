### External Checks Registry

Here is a list of external checks you can apply to your kubernetes cluster once you have Kuberhealthy installed. 

Make sure to add your check here:

| Check Name | Description | khcheck Resource | Contributor |
| --- | --- | --- | --- |
| [Daemonset Check](../cmd/daemonSetExternalCheck/README.md) | Ensures daemonsets can be successfully deployed | [daemonSetExternalCheck.yaml](../cmd/daemonSetExternalCheck/daemonSetExternalCheck.yaml) | @integrii @joshulyne |
| Deployment Test | Ensures that a Deployment and Service can be provisioned and created within the Kubernetes cluster | ???? | @jonnydawg |
| [Pod Restarts Check](../cmd/podRestartsExternalCheck/README.md) | Checks for excessive pod restarts in any namespace | [podRestartsExternalCheck.yaml](../cmd/podRestartsExternalCheck/podRestartsExternalCheck.yaml) | @integrii @joshulyne |
