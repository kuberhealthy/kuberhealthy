# metrics

The metrics package exports Kuberhealthy status information in Prometheus format. It assembles metric lines for cluster health, individual checks, controller statistics, run durations, and run counters, and provides helpers for writing metric responses.

Its focus is on generating metrics from `health.State` data. Collection of that state and HTTP serving are delegated to other parts of the system. Label allow/deny lists and truncation are enforced here to avoid unbounded cardinality.
