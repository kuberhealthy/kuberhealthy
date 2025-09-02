# Python Kuberhealthy Client

This directory contains a minimal Python application that demonstrates how to
report status back to [Kuberhealthy](https://github.com/kuberhealthy/kuberhealthy).
The example loads the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables
provided to checker pods and includes commented calls to `report_ok` and
`report_error`.

## Running the example

Set the `KH_REPORTING_URL` and `KH_RUN_UUID` environment variables, add your
check logic to `client.py`, and then run:

```bash
python3 client.py
```

Within the `main` function, uncomment either `report_ok()` or
`report_error("message")` after your logic depending on the result.

## Building and pushing the check

Use the provided `Makefile` and `Dockerfile` to build and publish the check
image.

```bash
make build IMG=myrepo/example-check:latest
make push IMG=myrepo/example-check:latest
```

## Using in your own checks

1. Add your check logic to `client.py` by replacing the placeholder in `main`.
   Call `report_ok()` when the check succeeds or `report_error("message")`
   when it fails.
2. Build and push your image as shown above.
3. Create a `KuberhealthyCheck` resource pointing at your image and apply it to any
   cluster where Kuberhealthy runs:

```yaml
apiVersion: kuberhealthy.github.io/v2
kind: KuberhealthyCheck
metadata:
  name: example-python-check
spec:
  image: myrepo/example-check:latest
  runInterval: 1m
```
