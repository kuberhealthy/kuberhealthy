## Knative service

This check tests if a Knative service can be created and available locally in your cluster. It will try to create the `Hello World` Knative service example (customizable) and will try 3 times (customizable) to reach it with a delay of 10 seconds (also customizable) between each attempt. The check is a request for a 200 return.
Wether successful or not the Knative service is then deleted.

This check tests if a Knative service can be created and available locally in your cluster. It will try to create a Knative service with the image provided and will try X times to reach it with a delay of Y seconds between each attempt. The check is a request for a 200 return.
Wether successful or not the Knative service is then deleted.

#### Check Configuration Environment Variables

- `KS_IMAGE`: image used by the Knative service
- `KS_NS`: namespace of the Knative service
- `KS_SVC`: name of the Knative service
- `KH_ATT`: Number of attempts 
- `KH_DEL`: Delay between two attempts 

#### Example KuberhealthyCheck Spec

Here is an example using the image from Knative sample `Hello World` and will create a Knative service named `knativesvc` in the namespace `kuberhealthy`. The check will try to reach it `3` times with `10` seconds between each attempt.

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: knativesvc
  namespace: kuberhealthy
spec:
  runInterval: 15m
  timeout: 2m
  podSpec:
    containers:
      - image: "kuberhealthy/knative-svc-check:v0.4"
        imagePullPolicy: Always
        name: knativesvc
        env:
          - name: KH_ATT
            value: "3"
          - name: KH_DEL
            value: "10"
          - name: KS_SVC
            value: "kh-knativesvc"
          - name: KS_NS
            value: "kuberhealthy"
          - name: KS_IMAGE
            value: "gcr.io/knative-samples/helloworld-go"
```
