package envs

// KHReportingURL is the environment variable used to tell external checks where to send their status updates
const KHReportingURL = "KH_REPORTING_URL"

// KHRunUUID is the environment variable used to tell external checks their check's UUID so that they
// can be de-duplicated on the server side.
const KHRunUUID = "KH_RUN_UUID"

// KHDeadline is the environment variable name for when checks must finish their runs by in unixtime
const KHDeadline = "KH_CHECK_RUN_DEADLINE"
