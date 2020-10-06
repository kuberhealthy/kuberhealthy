### External Jobs

External jobs are configured using the `khjob` custom resources.  These `khjobs` are just like `khchecks` but are configured without a runInterval as they run only once. They act like manually triggered k8s jobs, where as soon as your `khjob` resource is applied to the cluster, kuberhealthy runs it automatically. Any `khcheck` can be configured to be a `khjob` as long as you:
 1) Change the resource to `kind: KuberhealthyJob`
 2) Remove the `runInterval` in the `spec`

A list of pre-made checks that you can easily configure into `khjobs` are listed [in the external checks registry](../docs/EXTERNAL_CHECKS_REGISTRY.md).  

Every `khjob` is unique, you cannot retrigger the same `khjob`. To rerun a `khjob` you must delete the `khjob` resource and re-apply the `khjob` OR rename your `khjob` in `metdata.name`.

### `khjob` Anatomy

A `khjob` looks like this:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyJob
metadata:
  name: kh-test-job # the name of this job and the job pod
  namespace: kuberhealthy # the namespace the job pod will run in
spec:
  timeout: 2m # After this much time, Kuberhealthy will kill your job and consider it "failed"
  extraAnnotations: # Optional extra annotations your pod can have
    comcast.com/testAnnotation: test.annotation
  extraLabels: # Optional extra labels your pod can be configured with
    testLabel: testLabel
  podSpec: # The exact pod spec that will run.  All normal pod spec is valid here.
    containers:
    - env: # Environment variables are optional but a recommended way to configure job behavior
      - name: REPORT_FAILURE
        value: "false"
      - name: REPORT_DELAY
        value: 6s
      image: quay.io/comcast/test-external-check:latest # The image of the job you want to run.
      imagePullPolicy: Always # During job development, it helps to set this to 'Always' to prevent on-node image caching.
      name: main
      resources:
        requests:
          cpu: 10m
          memory: 50Mi
```
