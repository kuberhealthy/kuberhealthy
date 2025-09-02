# logruslogr

logruslogr provides an adapter between logrus and the `logr` logging interface. It allows components that expect a `logr.Logger` to emit structured logs through a configured logrus `Logger`.

The package focuses solely on translating log calls and managing key/value pairs. Configuration and lifecycle of the underlying logrus logger remain the caller's responsibility.
