# khcheck

khcheck defines the internal representation of a Kuberhealthy check. The `KHCheck` struct tracks scheduling parameters, runtime metadata, and cancellation controls for an individual check pod.

This package is limited to the data structure and related synchronization primitives. Logic that schedules or executes checks resides in the `kuberhealthy` controller package.
