# metrics

The metrics package exports Kuberhealthy status information in Prometheus format. It assembles metric lines for overall health, individual checks, run durations, and failure counts, and provides helpers for writing metric responses.

Its focus is on generating metrics from `health.State` data. Collection of that state and HTTP serving are delegated to other parts of the system.
