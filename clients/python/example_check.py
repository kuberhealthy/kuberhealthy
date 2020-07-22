from kh_client import *
import os

fail = os.getenv("FAIL", False)

if fail:
    print("Reporting failure.")
    try:
        report_failure(["example failure message"])
    except Exception as e:
        print(f"Error when reporting failure: {e}")
        exit(1)
else:
    print("Reporting success.")
    try:
        report_success()
    except Exception as e:
        print(f"Error when reporting success: {e}")
        exit(1)
exit(0)
