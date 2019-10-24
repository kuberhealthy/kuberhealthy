## Daemonset Check

The *Daemonset Check* ensures that Daemonsets can be deployed and terminated on all nodes in the cluster. When the spec 
is applied to your cluster, Kuberhealthy recognizes it as a KHCheck resource and provisions a checker pod to run the 
Daemonset Check. The Daemonset Check deploys a Daemonset, and waits for all the daemonset pods to be in the 'Ready' 
state, then terminates them and ensures all pod terminations were successful. 

The check runs every 15 minutes (spec.runInterval), with a check timeout set to 10 minutes (spec.timeout). If the check 
does not complete within the given timeout it will report a timeout error on the status page. The check takes in a 
CHECK_POD_TIMEOUT environment variable that ensures the clean up of rogue daemonsets or daemonset pods after the check
has finished within this timeout. 

Containers are deployed with their resource requirements set to 0 cores and 0 memory and use the pause container from 
Google (gcr.io/google_containers/pause:0.8.0), which is likely already cached on your nodes. The 
node-role.kubernetes.io/master NoSchedule taint is tolerated by daemonset testing pods. The pause container is already 
used by kubelet to do various tasks and should be cached at all times. If a failure occurs anywhere in the daemonset 
deployment or tear down, an error is shown on the status page describing the issue.

#### Daemonset Check Kube Spec:

```$xslt
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-daemonset-check
  namespace: kuberhealthy
spec:
  runInterval: 15m
  # Make sure this Kuberhealthy check timeout is GREATER THAN the daemonset checker timeout
  # set in the env var CHECK_POD_TIMEOUT. Default is set to 5m (5 minutes).
  timeout: 10m
  extraAnnotations:
    comcast.com/testAnnotation: test.annotation
  extraLabels:
    testLabel: testLabel
  podSpec:
    containers:
      - env:
          - name: POD_NAMESPACE
            value: "kuberhealthy"
          - name: CHECK_POD_TIMEOUT
            # Make sure this value is less than the Kuberhealthy check timeout.
            # Default is set to 10m (10 minutes).
            value: "5m"
        image: quay.io/comcast/kh-daemonset-check:1.0.0
        imagePullPolicy: Always
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### Daemonset Check Diagram

![](../../images/kh-ds-check.gif)


#### How-to

To implement the Daemonset Check with Kuberhealthy, apply the configuration file [daemonSetCheck.yaml](daemonSetCheck.yaml) to your Kubernetes Cluster.
Make sure you are using the latest release of Kuberhealthy 2.0.0. 

The configuration file contains:
- KuberhealthyCheck
- Role
- Rolebinding
- Service
- Secret

The role, rolebinding, service, and secret are all required to create and delete all daemonsets and pods from the check
in the given namespaces you apply the KHCheck to. The assumed default service account does not give you the permissions 
to do so. 

You can also create the service account directly by running: `kubectl create serviceaccount daemonset-khcheck`. 
This will create and apply the necessary secret to your service account. Once you've created the service account, make 
sure to apply the rest of the configurations for the Role, RoleBinding, and Kuberhealthy check. 
