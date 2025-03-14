package envs

// KHReportingURL is the environment variable used to tell external checks where to send their status updates
const KHReportingURL = "KH_REPORTING_URL"

// KHRunUUID is the environment variable used to tell external checks their check's UUID so that they
// can be de-duplicated on the server side.
const KHRunUUID = "KH_RUN_UUID"

// KHDeadline is the environment variable name for when checks must finish their runs by in unixtime
const KHDeadline = "KH_CHECK_RUN_DEADLINE"

// // KHPodNamespace is the namespace variable used to tell external checks their namespace to perform
// // checks in.
// const KHPodNamespace = "KH_POD_NAMESPACE"

// KHCheckNameAnnotationKey is the annotation which holds the check's name for later validation when the pod calls in
const KHCheckNameAnnotationKey = "kuberhealthy.github.io/check-name"

// // DefaultKuberhealthyReportingURL is the default location that external checks
// // are expected to report into.
// const DefaultKuberhealthyReportingURL = "http://kuberhealthy.kuberhealthy.svc.cluster.local/externalCheckStatus"

// // defaultTimeout is the default time a pod is allowed to run when this checker is created
// const defaultTimeout = time.Minute * 15

// // defaultShutdownGracePeriod is the default time a pod is given to shutdown gracefully
// const defaultShutdownGracePeriod = time.Minute
