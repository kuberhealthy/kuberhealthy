import dataclasses
import json
import os
from dataclasses import dataclass
from typing import List

import requests
from requests import HTTPError


@dataclass
class StatusReport:
    Errors: List[str]
    OK: bool


def report_success():
    report = StatusReport(Errors=[], OK=True)
    try:
        send_report(report)
    except Exception as e:
        raise Exception(f"failed to send report: {e}")


def report_failure(error_messages: List[str]):
    report = StatusReport(Errors=error_messages, OK=False)
    try:
        send_report(report)
    except Exception as e:
        raise Exception(f"failed to send report: {e}")


def get_kuberhealthy_url():
    try:
        reporting_url_env = os.environ["KH_REPORTING_URL"]
    except:
        raise Exception("fetched KH_REPORTING_URL environment variable but it was blank")
    return reporting_url_env


def send_report(status_report: StatusReport):
    try:
        data = json.dumps(dataclasses.asdict(status_report))
    except Exception as e:
        raise Exception(f"failed to convert status report to json string: {e}")

    try:
        kh_url = get_kuberhealthy_url()
    except Exception as e:
        raise Exception(f"failed to fetch the kuberhealthy url: {e}")

    response = requests.post(kh_url, data=data, headers={"Content-Type": "application/json"})
    try:
        response.raise_for_status()
    except HTTPError as e:
        raise Exception(f"got a bad status code from kuberhealthy: {response.status_code}")


if __name__ == '__main__':
    report_success()
    report_failure(["check is failed"])
