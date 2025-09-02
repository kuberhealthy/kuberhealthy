# envs

This package centralizes the names of environment variables used by Kuberhealthy check pods. Constants such as `KH_REPORTING_URL`, `KH_RUN_UUID`, and `KH_CHECK_RUN_DEADLINE` are defined here for reuse across packages.

Its scope is limited to providing canonical variable names so that other packages can reference them without duplicating string literals or parsing logic.
