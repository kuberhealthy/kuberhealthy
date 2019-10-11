### External Checks Registry

Here is a list of external checks you can apply to your kubernetes cluster once you have Kuberhealthy installed. 

Make sure to add your check here:

| Check Name | Description | khcheck Resource | Contributor |
| --- | --- | --- | --- |
| Daemonset Test | Ensures daemonsets can be successfully deployed | [daemonSetExternalCheck.yaml](../cmd/daemonSetExternalCheck/daemonSetExternalCheck.yaml) | @integrii @joshulyne |
| Deployment Test | Ensures load balancer can be hit successfully | ???? | @jonnydawg |
