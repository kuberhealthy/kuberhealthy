## Kops AMI Check

This check looks to see if the images used for Kops instance groups still exist in the AWS Amazon marketplace. Pulls object contents from AWS S3 and parses them into Kops-instance-group structs and then pulls a list of available images from AWS EC2 AMI marketplace. Images used for these Kops-instance-groups are checked against the available list of AMIs to ensure that the instance-group image is available for new nodes.

#### Check Steps

This check follows the list of actions in order during the run of the check:
1.  Queries bucket object contents from AWS S3 for the Kops state store.
2.  Queries available images from AWS EC2.
3.  Checks both lists to make sure that they exist.

#### Check Details

- Namespace: kuberhealthy
- Timeout: 15 minutes
- Check Interval: 10 minutes
- Check name: `kh-ami-check`
- Configurable check environment variables:
  - `AWS_REGION`: The region that the S3 bucket exists in. (Although S3 buckets are global, their creation location matters.)
  - `AWS_S3_BUCKET_NAME`: The name of the S3 bucket for the Kops state store.
  - `CLUSTER_FQDN`: The fully qualified domain name for the cluster.

#### Example KuberhealthyCheck Spec

```yaml
---
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-ami-check
  namespace: kuberhealthy
spec:
  runInterval: 10m
  timeout: 15m
  extraAnnotations:
    iam.amazonaws.com/role: <role-arn>
  podSpec:
    containers:
    - name: kh-ami-check
      image: 
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

To use the *Deployment Check* with Kuberhealthy, apply the configuration file [ami-check.yaml](ami-check.yaml) to your Kubernetes Cluster.
Make sure you are using the latest release of Kuberhealthy 2.0.0. 

The check configuration file contains:
- KuberhealthyCheck
