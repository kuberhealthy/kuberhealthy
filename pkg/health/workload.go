package health

// KHWorkload is used to describe the different types of kuberhealthy workloads: KhCheck or KHJob
type KHWorkload string

// Two types of KHWorkloads are available: Kuberhealthy Check or Kuberhealthy Job
// KHChecks run on a scheduled run interval
// KHJobs run once
const (
	KHCheck KHWorkload = "KHCheck"
	KHJob   KHWorkload = "KHJob"
)
