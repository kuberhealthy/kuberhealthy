## Deployment and Service

This check tests if a `deployment` and `service` can be created within your Kubernetes cluster. It will attempt to bring up a `deployment` with `2` replicas and a `service` of type `LoadBalancer` in the `kuberhealthy` namespace and waits for the pods to come up. Once the `deployment` is ready, the check makes a request to the hostname looking for a `200 OK`. The check then proceeds to terminate them and ensures that the deployment and service terminations were successful. A complete tear down of the `deployment` and `service` after receiving a `200 OK` marks a successful test run.

Container resource requests are set to `15 millicores` of CPU and `20Mi` units of memory and use the Nginx's latest image `nginx:latest` for the deployment. If the environment variable `CHECK_DEPLOYMENT_ROLLING_UPDATE` is set to `true`, the check will attempt to perform a rolling-update on the `deployment`. Once this rolling-update completes, the check makes another request to the hostname looking fora `200 OK` again before cleaning up. By default, the check will initially deploy Nginx's `nginx:latest` image, and update to `nginx:alpine`. 

Custom images can be used for this check and can be specified with the `CHECK_IMAGE` and `CHECK_IMAGE_ROLL_TO` environment variables. If a custom image requires the use of environment variables, they can be passed down into your container by setting the environment variable `ADDITIONAL_ENV_VARS` to a string of comma-separated values (`"X=foo,Y=bar"`).

The number of replicas the `deployment` brings up can be adjusted with the `CHECK_DEPLOYMENT_REPLICAS` environment variable. By default the amount of replicas used is `2`, but this can be customized for different scenarios and environments. `maxSurge` and `maxUnavailable` values for the deployment is calculated to be %50 of the deployment replicas (rounded-up).

A successful run implies that a deployment and service can be brought up and the corresponding hostname endpoint returns a `200 OK` response.  A failure implies that an error occurred anywhere in the deployment creation, service creation, HTTP request, or tear down process -- resulting in an error report to the _Kuberhealthy_ status page.

- Namespace: kuberhealthy
- Timeout: 5 minutes
- Check Interval: 30 minutes
- Check name: `kh-deployment-check`
- Configurable check environment variables:
  - `CHECK_IMAGE`: Initial container image. (default=`nginx:latest`)
  - `CHECK_IMAGE_ROLL_TO`: Container image to roll to. (default=`nginx:alpine`)
  - `CHECK_DEPLOYMENT_NAME`: Name for the check's deployment. (default=`kh-deployment-check-deployment`)
  - `CHECK_SERVICE_NAME`: Name for the check's service. (default=`kh-deployment-check-service`)
  - `CHECK_NAMESPACE`: Namespace for the check (default=`kuberhealthy`).
  - `CHECK_DEPLOYMENT_REPLICAS`: Number of replicas in the deployment (default=`2`).
  - `CHECK_TIME_LIMIT_SECONDS`: Number of seconds the check will allow itself before timing out.
  - `CHECK_DEPLOYMENT_ROLLING_UPDATE`: Boolean to enable rolling-update (default=`false`).
  - `ADDITIONAL_ENV_VARS`: Comma separated list of `key=value` variables passed into the pod's containers.
  - `SHUTDOWN_GRACE_PERIOD_SECONDS`: Amount of time in seconds the shutdown will allow itself to clean up after an interrupt signal. (default=`30s`)


#### Example KuberhealthyCheck Spec

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-deployment-check
  namespace: kuberhealthy
spec:
  runInterval: 30m
  timeout: 7m
  podSpec:
    containers:
    - name: kh-deployment-check
      image: quay.io/comcast/deployment-check:1.0.0
      imagePullPolicy: IfNotPresent
      env:
        - name: CHECK_IMAGE
          value: "nginx:1.17-perl"
        - name: CHECK_IMAGE_ROLL_TO
          value: "nginx:1.17.5-perl"
        - name: CHECK_DEPLOYMENT_REPLICAS
          value: "4"
        - name: CHECK_TIME_LIMIT_SECONDS
          value: "300"
        - name: CHECK_DEPLOYMENT_ROLLING_UPDATE
          value: "true"
        - name: ADDITIONAL_ENV_VARS
          value: "var1=foo,var2=bar"
      resources:
        requests:
          cpu: 25m
          memory: 15Mi
        limits:
          cpu: 40m
      restartPolicy: Always
```

#### Install

To use the *Deployment Check* with Kuberhealthy, apply the configuration file [deployment-check.yaml](deployment-check.yaml) to your Kubernetes Cluster.
Make sure you are using the latest release of Kuberhealthy 2.0.0. 

The check configuration file contains:
- KuberhealthyCheck
- Role
- Rolebinding
- ServiceAccount

The role, rolebinding, and service account are all required to create and delete all deployments and services from the check in the given namespaces you install the check for. The assumed default service account does not provide enough permissions for this check to run. 

You can also create the service account directly by running: `kubectl create serviceaccount deployment-khcheck`. 
This will create and apply the necessary secrets and tokens to your service account. Once you've created the service account, make 
sure to apply the rest of the configurations for the Role, RoleBinding, and Kuberhealthy check. 