# health

The health package models Kuberhealthy's overall status and individual check results. It defines structures such as `State` and `CheckDetail`, helpers for appending errors, and utilities for writing JSON responses. It also includes the lightweight `Report` type used by external checks to communicate their outcomes.

Its responsibilities are limited to representing health data and formatting it for consumption. Execution of checks and persistence of results are handled by other packages.
