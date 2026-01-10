# Check Client

This package reports an external check's status back to Kuberhealthy using details supplied via environment variables.

- `ReportSuccess()` posts a successful check result with no errors.
- `ReportFailure(errorMessages []string)` posts a failing result with the given error messages.
- `GetDeadline()` returns the time by which the check must finish from `KH_CHECK_RUN_DEADLINE`.

The client reads connection information like `KH_REPORTING_URL` (must point to `/check`) and `KH_RUN_UUID` and automatically retries submissions with exponential backoff.
