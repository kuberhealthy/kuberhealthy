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

package masterCalculation

import (
	"os"
	"testing"

	"github.com/Comcast/kuberhealthy/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
)

var kubeConfigFile = os.Getenv("HOME") + "/.kube/config"

func TestRun(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	master, err := CalculateMaster(client)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(master)
}
