#### Deployment and Service

The check deploys a `deployment` to the `kuberhealthy` namespace with `2` replicas and waits for pods and service to be in the 'Ready' state and serve a hostname. The service created in this check is of type `LoadBalancer` NOT `ClusterIP`. Once ready, the check makes a request to the endpoint for a `200 OK` response and then proceeds to terminate them and ensures that the deployment and service terminations were successful. Containers are deployed with their resource requesets set to 15 millicores and 20Mi units of memory and uses the latest Nginx web server container (`nginx:latest`) for the deployment. A successful run implies that a deployment and service can be brought up and the corresponding hostname endpoint returns a `200 OK` response. If `CHECK_DEPLOYMENT_ROLLING_UPDATE` is set to `true`, the check will initially deploy Nginx's `alpine` tag, roll to the latest (`nginx:alpine` -> `nginx:latest`), and look for a second `200 OK` response. A failure implies that an error occurred anywhere in the deployment creation, service creation, HTTP request, or tear down process -- resulting in an error report to the Kuberhealthy status page.

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
  - `ADDITIONAL_ENV_VARS`: `Key`=`Pair` delimited list of variables passed into the pod's containers.
  - `SHUTDOWN_GRACE_PERIOD_SECONDS`: Amount of time in seconds the shutdown will allow itself to clean up after an interrupt signal. (default=`30s`)

