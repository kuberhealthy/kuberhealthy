## KIAM Check

This check is intended for [`KIAM`](https://github.com/uswitch/kiam) users __only__.

The [`KIAM`](https://github.com/uswitch/kiam) check tests that `KIAM` servers and agents running within your cluster can properly intercept AWS metadata service requests. This check queries Lambdas utilizing a `KIAM` annotation, which can be set via your `KuberhealthyCheck` custom resource by passing in a field under `spec`:

```yaml
spec:
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn>
```

The check will report a success if it is able to list any amount of Lambda configurations from AWS; otherwise it will report a failure.

#### Example KuberhealthyCheck Spec

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kiam
  namespace: kuberhealthy
spec:
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn> # Replace this value with your ARN
  runInterval: 5m
  timeout: 15m
  podSpec:
    containers:
    - name: kiam
      image: kuberhealthy/kiam-check:v1.3.0
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

You must first configure a valid role ARN via IAM in your AWS account. The ARN needs to have enough permissions to perform reads on the Lambda service. If you need to create or update a role for this, attaching the AWS-provided `AWSLambdaReadOnlyAccess` policy should allow enough permissions for the check to run.

To use the *KIAM Check* with Kuberhealthy, replace the `<role-arn>` value in the configuration file at [kiam-check](kiam-check.yaml). Then apply it to your Kubernetes Cluster `kubectl apply -f kiam-check.yaml`.

The check configuration file contains:
- KuberhealthyCheck

The role, rolebinding, and service account are all required to create and delete all deployments and services from the check in the given namespaces you install the check for. The assumed default service account does not provide enough permissions for this check to run.
