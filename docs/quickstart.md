# Quick Start

This guide should create a production ready instance of Kuberhealthy ready for your cluster.  

## Security Considerations:

Kuberhealthy exposes an inscure (non-HTTPS) endpoint without authentication.  You should never expose this endpoint to the public internet.  Exposing Kuberhealthy to the internet could result in private cluster information being exposed to the public internet when errors occur.



## Setup Steps:

1. In the Service section of `kuberhealthy.yaml`, there is a line `external-dns.alpha.kubernetes.io/hostname: "kuberhealthy.k8s.company.com"`. If you are using `https://github.com/kubernetes-incubator/external-dns`, replace this value with where you want your endpoint to be located.
2. With system master permissions to the cluster, run `kubectl create -f https://github.com/Comcast/kuberhealthy/kuberhealthy.yaml`. This will create a Namespace, Service Account, Deployment, Pod Disruption Budget, Service, Custom Resource Definition, Role, and Role Binding , which are all needed for Kuberhealthy to operate.  Kuberhealthy will exist entirely in its own namespace, `kuberhealthy`.
3. When Kuberhealthy is up and running, you should be able to visit your endpoint and get the status of your cluster.
