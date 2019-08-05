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

package khcheckcrd

import (
	"os"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const testCheckName = "gotest"

var kubeConfigFile = os.Getenv("HOME") + "/.kube/config"

func TestClient(t *testing.T) {
	_, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreate(t *testing.T) {

	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	checkDetails := NewKuberhealthyCheck(testCheckName, NewCheckConfig())
	result, err := client.Create(&checkDetails, resource)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", result)
}

func TestList(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.List(metav1.ListOptions{}, resource)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Get(metav1.GetOptions{}, resource, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Kind)
}

// createTestCheck creates a test placeholder check
func createTestCheck(checkName string) error {
	check := NewKuberhealthyCheck(testCheckName, NewCheckConfig())

	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		return err
	}

	_, err = client.Create(&check, resource)
	return err

}

func TestUpdate(t *testing.T) {

	// make client
	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}

	// ensure that the resource exists on the testing server
	_ = createTestCheck(testCheckName)

	// get the custom resource for the check named gotest
	checkConfig, err := client.Get(metav1.GetOptions{}, resource, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", checkConfig)

	// change something in the check config
	checkConfig.Spec.RunInterval = time.Minute * 4
	randomUUID := randomdata.RandStringRunes(15)
	checkConfig.Spec.CurrentUUID = randomUUID
	t.Logf("%+v", checkConfig)

	// apply the updated version to the server
	_, err = client.Update(checkConfig, resource, testCheckName)
	if err != nil {
		t.Fatal(err)
	}

	// get the updated version back
	result, err := client.Get(metav1.GetOptions{}, resource, testCheckName)
	if err != nil {
		t.Fatal(err)
	}

	// ensure the interval is set to what we wanted
	if result.Spec.RunInterval != time.Minute*4 {
		t.Log("Incorrect name after updating and fetching CRD.  Wanted", time.Minute*4, "but got", result.Spec.RunInterval)
		t.Fail()
	}

	// ensure the UUID is the oen we set
	if result.Spec.CurrentUUID != randomUUID {
		t.Log("Incorrect CurrentUUID state after updating and fetching CRD.  Wanted", randomUUID, "but got", result.Spec.CurrentUUID)
		t.Fail()
	}
	t.Log(result.Kind)
}

func TestDelete(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	check, err := client.Get(metav1.GetOptions{}, resource, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Delete(resource, check.Name)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", result)
}
