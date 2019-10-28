## KIAM Check

This check tests `KIAM` servers and agents running within your cluster can properly intercept AWS metadata service requests. This check utilizes a `KIAM` annotation for querying Lambdas which can be set via your `KuberhealthyCheck` custom resource by passing in a field under `spec` like so:

```yaml
spec:
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn>
```

The check will report a success if it is able to list any amount of Lambda configurations from AWS; otherwise it will report a failure.

#### Check Details

- Namespace: kuberhealthy
- Timeout: 5 minutes 30 seconds
- Check Interval: 5 minutes
- Check name: `kh-kiam-check`

#### Example KuberhealthyCheck Spec

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-kiam-check
  namespace: kuberhealthy
spec:
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn>
  runInterval: 5m
  timeout: 5m30s
  podSpec:
    containers:
    - name: kh-kiam-check
      image: quay.io/comcast/kiam-check:1.0.0
      imagePullPolicy: IfNotPresent
      env:
      - name: AWS_REGION
        value: us-west-2
      resources:
        requests:
          cpu: 15m
          memory: 10Mi
        limits:
          cpu: 30m
      restartPolicy: Always 
```

#### Install

To use the *Deployment Check* with Kuberhealthy, apply the configuration file [kiam-check.yaml](kiam-check.yaml) to your Kubernetes Cluster.
Make sure you are using the latest release of Kuberhealthy 2.0.0. 

The check configuration file contains:
- KuberhealthyCheck

The role, rolebinding, and service account are all required to create and delete all deployments and services from the check in the given namespaces you install the check for. The assumed default service account does not provide enough permissions for this check to run.