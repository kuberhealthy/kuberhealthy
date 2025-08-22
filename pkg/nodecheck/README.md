# Node Check Utilities

This package contains helpers for Kuberhealthy check pods that run on Kubernetes nodes.

- `EnableDebugOutput` switches the package's logging level to debug.
- `WaitForKuberhealthy(ctx)` waits until the Kuberhealthy reporting URL (from `KH_REPORTING_URL`) is reachable before a check proceeds.
- `WaitForNodeAge(ctx, client, nodeName, minNodeAge)` ensures the node has existed for at least `minNodeAge` before the check continues.

These functions allow a check pod to delay execution until Kuberhealthy's endpoint responds and the hosting node is old enough to be considered stable.
