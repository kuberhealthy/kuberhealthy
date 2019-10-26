## Pod Restarts Check

The *Pod Restarts Check* checks for excessive pod restarts in a given `POD_NAMESPACE`. When the spec is applied to your 
cluster, Kuberhealthy recognizes it as a KHCheck resource and provisions a checker pod to run the Pod Restarts Check. 
The Pod Restarts Check deploys a pod that loops through all the pods in the given `POD_NAMESPACE` and runs a ticker that 
keeps count of pod restarts within the given time frame `CHECK_RUN_WINDOW`. Pod restart counts that exceed the 
`MAX_FAILURES_ALLOWED` within the given time fram are reported back as check failures. 

In the example below, the check runs every hour (spec.runInterval), with a check timeout set to 58 minutes 
(spec.timeout), and a `CHECK_RUN_WINDOW` of 55 minutes. The check runs for 55 minutes, and every minute, the pod reports 
back either a failure or success depending on whether or not it found pods with restart counts that exceeded the 
`MAX_FAILURES_ALLOWED` count. If the check does not complete within the given timeout it will report a timeout error on 
the status page. 

#### Pod Restarts Check Kube Spec:

```$xslt
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: pod-restarts-check
  namespace: kuberhealthy
spec:
  runInterval: 60m
  timeout: 58m
  podSpec:
    containers:
      - env:
          - name: POD_NAMESPACE
            value: "kube-system"
          - name: CHECK_RUN_WINDOW
            value: "55m"
          - name: MAX_FAILURES_ALLOWED 
            ## Default is set to 5.
            value: "5"
        image: quay.io/comcast/pod-restarts-check:1.0.0
        imagePullPolicy: Always
        name: main
        resources:
          requests:
            cpu: 10m
            memory: 50Mi
```

#### How-to 

To implement the Pod Restarts Check with Kuberhealthy, run:
 
`kubectl apply -f https://raw.githubusercontent.com/Comcast/kuberhealthy/2.0.0/cmd/podRestartsCheck/podRestartsCheck.yaml`

Make sure you are using the latest release of Kuberhealthy 2.0.0. 
