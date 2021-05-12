### client-python

This is a `Python` client for Kuberhealthy external checks. This client exports functions for sending status
report POST requests to Kuberhealthy. This is a `Python` implementation of the `JavaScript` client found [here](../js/kh-client.js).
This package needs Python version >=3.7 (for using dataclasses)

##### Usage

Download the client into your Python project by navigating to your project directory and downloading the client file:

```shell
cd my-kh-check
curl -O -L https://raw.githubusercontent.com/kuberhealthy/kuberhealthy/master/clients/python/kh_client.py
```

In your project, require the file you just downloaded:

```python
from kh_client import *
```

Then you can report check status to Kuberhealthy using `report_success()` or `report_failure()`:

```python
# Report failure. 
report_failure(["example failure message"])

# Report success.
report_success()
```

##### Example Use

```python
try: 
    report_success()
except Exception as e:
    print(f"Error when reporting success: {e}")
```