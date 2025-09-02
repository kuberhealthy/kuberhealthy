#!/usr/bin/env python3
"""Example Kuberhealthy client in Python."""

import json
import os
import urllib.request


KH_REPORTING_URL = "KH_REPORTING_URL"
KH_RUN_UUID = "KH_RUN_UUID"


def _get_env(name: str) -> str:
    """Return the value of the environment variable *name* or raise an error."""
    value = os.getenv(name)
    if not value:
        raise EnvironmentError(f"{name} must be set")
    return value


def _post_status(payload: dict) -> None:
    """Send *payload* to the Kuberhealthy reporting URL."""
    url = _get_env(KH_REPORTING_URL)
    run_uuid = _get_env(KH_RUN_UUID)
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        url,
        data=data,
        headers={"content-type": "application/json", "kh-run-uuid": run_uuid},
    )
    with urllib.request.urlopen(request, timeout=10) as response:  # nosec B310
        response.read()


def report_ok() -> None:
    """Report a successful check to Kuberhealthy."""
    _post_status({"OK": True, "Errors": []})


def report_error(message: str) -> None:
    """Report a failure to Kuberhealthy with *message* as the error."""
    _post_status({"OK": False, "Errors": [message]})


def main() -> None:
    """Run the example client."""
    # INSERT YOUR CHECK LOGIC HERE
    # report_ok()
    # report_error("something went wrong")
    pass


if __name__ == "__main__":
    main()
