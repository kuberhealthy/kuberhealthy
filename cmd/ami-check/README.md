## kops AMI Check

This check looks to see if the images used for kops instance groups still exist in the AWS Amazon marketplace. The check
pulls object contents from AWS S3, parsing them into kops-instance-group structs and then pulls a list of available
images from the AWS EC2 AMI marketplace. Images used for these kops-instance-groups are checked against the available
list of AMIs to ensure that the instance-group image is available for new nodes.

#### Check Steps

1.  Queries bucket object contents from AWS S3 for the kops state store.
2.  Queries available images from AWS EC2.
3.  Verifies that images grabbed from AWS S3 exist amongst the images grabbed from the AWS EC2 AMI marketplace.

#### Check Details

- Namespace: kuberhealthy
- Timeout: 15 minutes
- Check Interval: 10 minutes
- Check name: `ami`
- Configurable check environment variables:
  - `AWS_REGION`: The region that the S3 bucket exists in. (Although S3 buckets are global, their creation location matters.)
  - `AWS_S3_BUCKET_NAME`: The name of the S3 bucket for the kops state store.
  - `CLUSTER_FQDN`: The fully qualified domain name for the cluster.

#### Example KuberhealthyCheck Spec

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: ami
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 15m
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn> # Replace this value with your ARN
  podSpec:
    containers:
      - name: ami
        image: kuberhealthy/ami-check:v1.4.0
        imagePullPolicy: IfNotPresent
        env:
          - name: AWS_REGION
            value: "us-east-1"
          - name: AWS_S3_BUCKET_NAME
            value: "s3-bucket-name"
          - name: CLUSTER_FQDN
            value: "cluster.k8s"
        resources:
          requests:
            cpu: 10m
            memory: 10Mi
          limits:
            cpu: 15m
    restartPolicy: Always
```

#### Install

You must first configure a valid role ARN via IAM in your AWS account. The ARN needs to have enough permissions to perform reads on the Lambda service. If you need to create or update a role for this, attaching the AWS-provided `AmazonEC2ContainerRegistryReadOnly`, `AmazonEC2ReadOnlyAccess`, `AmazonS3ReadOnlyAccess` policies should allow enough permissions for the check to run.

To use the _kops AMI Check_ with Kuberhealthy, replace the `<role-arn>` value in the configuration file at [ami-check](ami-check.yaml). Then apply it to your Kubernetes Cluster `kubectl apply -f kiam-check.yaml`.`

The check configuration file contains:

- KuberhealthyCheck
