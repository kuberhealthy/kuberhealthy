// Copyright 2018 Comcast Cable Communications Management, LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package health

import (
	"time"
)

// WorkloadDetails contains details about a single kuberhealthy check or job's current status
type WorkloadDetails struct {
	OK               bool
	Errors           []string
	RunDuration      string
	Namespace        string
	LastRun          time.Time 		// the time the check last was last run
	AuthoritativePod string    		// the pod that last ran the check
	CurrentUUID      string    		`json:"uuid"` // the UUID that is authorized to report statuses into the kuberhealthy endpoint
	KHWorkload       KHWorkload		`json:"KHWorkload,omitempty"`
}

// NewWorkloadDetails creates a new WorkloadDetails struct
func NewWorkloadDetails() WorkloadDetails {
	return WorkloadDetails{
		Errors: []string{},
	}
}

// KHWorkload is used to describe the different types of kuberhealthy workloads: KhCheck or KHJob
type KHWorkload string

// Two types of KHWorkloads are available: Kuberhealthy Check or Kuberhealthy Job
// KHChecks run on a scheduled run interval
// KHJobs run once
const (
	KHCheck KHWorkload = "KHCheck"
	KHJob KHWorkload = "KHJob"
)
