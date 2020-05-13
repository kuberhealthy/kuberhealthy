## Creating Your Own `khcheck`

### Using Go 

Creating your own `khcheck` is very easy.  If you are using Go, we have an easy to use client package at [github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient](https://godoc.org/github.com/Comcast/kuberhealthy/v2/pkg/checks/external/checkclient).

<img src="../images/example check.png">

An example check with working Dockerfile is available to use as an example [here](../cmd/test-external-check/main.go).

### Using JavaScript

If you would like to write a check in JavaScript, there is a client for external checks found [here](../clients/js/).

There is also an [example check](../clients/js/example) in the under the same folder with a Dockerfile for reference.

For more information on the external client for JavaScript go [here](../clients/js/README.md).

### Using Another Language

Your check only needs to do a few things:

- Read the `KH_REPORTING_URL` environment variable.
- Send a `POST` to the `KH_REPORTING_URL` with the following JSON body:
- Ensure that your check finishes within the [unixtime](https://en.wikipedia.org/wiki/Unix_time) deadline specified in the `KH_CHECK_RUN_DEADLINE` environment variable. If `KH_CHECK_RUN_DEADLINE` is not respected, your check may run into a `400` error when reporting its state to Kuberhealthy. 

```json
{
  "Errors": [
    "Error 1 here",
    "Error 2 here"
  ],
  "OK": false
}
```

Never send `"OK": true` if `Errors` has values or you will be given a `400` return code.

Simply build your program into a container, `docker push` it to somewhere your cluster has access and craft a `khcheck` resource to enable it in your cluster where Kuberhealthy is installed.

Clients outside of Go can be found in the [clients directory](../clients).

#### Injected Check Pod Environment Variables
The following environment variables are injected into every checker pod that Kuberhealthy runs.  When writing your checker code, you can depend on these environment variables always being available to you, even if you do not specify them in your `khcheck` spec.
```
KH_REPORTING_URL: The Kuberhealthy URL to send POST requests to for check statuses.
KH_CHECK_RUN_DEADLINE: The Kuberhealthy-calculated deadline for checks given in unix.
```

### Creating Your `khcheck` Resource

Every check needs a `khcheck` to enable and configure it.  As soon as this resource is applied to the cluster, Kuberhealthy will begin running your check.  Whenever you make a change, Kuberhealthy will automatically re-load the check and restart any checks currently in progress gracefully.

Here is a minimal `khcheck` resource to start hacking with:

```yaml
apiVersion: comcast.github.io/v1
kind: KuberhealthyCheck
metadata:
  name: kh-test-check 
spec:
  runInterval: 30s # The interval that Kuberhealthy will run your check on 
  timeout: 2m # After this much time, Kuberhealthy will kill your check and consider it "failed"
  podSpec: # The exact pod spec that will run.  All normal pod spec is valid here.
    containers:
    - env: # Environment variables are optional but a recommended way to configure check behavior
        - name: MY_OPTION_ENV_VAR
          value: "option_setting_here"
      image: quay.io/comcast/test-external-check:latest # The image of the check you just pushed
      imagePullPolicy: Always # During check development, it helps to set this to 'Always' to prevent on-node image caching.
      name: main
```

That's it!  As soon as this `khcheck` is applied, Kuberhealthy will begin running your check, serving prometheus metrics for it, and displaying status JSON on the status page.

### Contribute Your Check

You can see a list of checks that others have written on the [check registry](EXTERNAL_CHECKS_REGISTRY.md).  If you have a check that may be useful to others and want to contribute, consider adding it to the registry!  Just fork this repository and send a PR.  This is made easy by simply checking the `Edit` pencil on the check registry page.
