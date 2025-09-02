# Ruby Client Example

This directory contains a minimal Ruby client for creating Kuberhealthy external checks.
The script reads the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables that
Kuberhealthy sets on checker pods and uses them to report status back to the
Kuberhealthy service.

## Using this example

1. Add your own check logic to `exampleCheck.rb`.
2. Build the container image:
   ```sh
   make build IMAGE=your-registry/ruby-kh-check:latest
   ```
3. Push the image to your registry:
   ```sh
   make push IMAGE=your-registry/ruby-kh-check:latest
   ```
4. Create a `KuberhealthyCheck` manifest that uses your image and apply it to the
   cluster where Kuberhealthy runs:
   ```yaml
   apiVersion: comcast.github.io/v1
   kind: KuberhealthyCheck
   metadata:
     name: ruby-example-check
     namespace: kuberhealthy
   spec:
     runInterval: 1m
     timeout: 30s
     podSpec:
       containers:
       - name: ruby-example
         image: your-registry/ruby-kh-check:latest
         imagePullPolicy: IfNotPresent
   ```

When the check pod starts, Kuberhealthy injects `KH_RUN_UUID` and `KH_REPORTING_URL` into
its environment. The example script reports success by default and reports failure with a
message if an exception is raised.
