## KIAM Check

The `KIAM` check tests that `KIAM` servers and agents running within your cluster can properly intercept AWS metadata service requests. AWS Lambdas are utilized for various event triggers and can be utilized for monitoring and alerting. This check queries Lambdas utilizing a `KIAM` annotation, which can be set via your `KuberhealthyCheck` custom resource by passing in a field under `spec`:

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
    iam.amazonaws.com/role: <role-arn> # Replace this value with your ARN
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

You must first configure a valid role ARN via IAM in your AWS account. The ARN needs to have enough permissions to perform reads on the Lambda service. If you need to create or update a role for this, attaching the AWS-provided `AWSLambdaReadOnlyAccess` policy should allow enough permissions for the check to run.

To use the *Deployment Check* with Kuberhealthy, replace the `<role-arn>` value in the configuration file at [kiam-check](kiam-check.yaml). Then apply it to your Kubernetes Cluster `kubectl apply -f kiam-check.yaml`. 

Make sure you are using the latest release of Kuberhealthy 2.0.0. 

The check configuration file contains:
- KuberhealthyCheck

The role, rolebinding, and service account are all required to create and delete all deployments and services from the check in the given namespaces you install the check for. The assumed default service account does not provide enough permissions for this check to run.
