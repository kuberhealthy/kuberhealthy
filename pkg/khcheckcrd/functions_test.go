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
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

const testCheckName = "gotest"

var kubeConfigFile = os.Getenv("HOME") + "/.kube/config"

func TestClient(t *testing.T) {
	_, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
}

// loadBasicCheckerPod loads a khcheck example spec from disk for testing
func loadBasicCheckerPod(filename string) (KuberhealthyCheck, error) {
	var c KuberhealthyCheck
	f, err := os.Open(filename)
	if err != nil {
		return c, err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return c, err
	}
	err = yaml.Unmarshal(b, &c)
	return c, err
}

func TestCreate(t *testing.T) {

	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	checkDetails := NewKuberhealthyCheck(testCheckName, defaultNamespace, NewCheckConfig(time.Second, v1.PodSpec{}))
	result, err := client.Create(&checkDetails, resource, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", result)
}

func TestList(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.List(metav1.ListOptions{}, resource, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Get(metav1.GetOptions{}, resource, defaultNamespace, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Kind)
}

func TestUpdate(t *testing.T) {

	// make client
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}

	// get the custom resource for the check named gotest
	checkConfig, err := client.Get(metav1.GetOptions{}, resource, defaultNamespace, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", checkConfig)

	// change something in the check config
	checkConfig.Spec.RunInterval = (time.Minute * 4).String()
	randomUUID := randomdata.RandStringRunes(15)
	checkConfig.Spec.CurrentUUID = randomUUID
	t.Logf("%+v", checkConfig)

	// apply the updated version to the server
	_, err = client.Update(checkConfig, resource, defaultNamespace, testCheckName)
	if err != nil {
		t.Fatal(err)
	}

	// get the updated version back
	result, err := client.Get(metav1.GetOptions{}, resource, defaultNamespace, testCheckName)
	if err != nil {
		t.Fatal(err)
	}

	// ensure the interval is set to what we wanted
	if result.Spec.RunInterval != "4m0s" {
		t.Log("Incorrect duration after updating and fetching CRD.  Wanted 4m0s but got", result.Spec.RunInterval)
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
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	check, err := client.Get(metav1.GetOptions{}, resource, defaultNamespace, testCheckName)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Delete(resource, check.Name, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", result)
}
