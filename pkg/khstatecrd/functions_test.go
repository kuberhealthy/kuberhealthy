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

package khstatecrd

import (
	"os"
	"testing"
	"time"

	"github.com/Comcast/kuberhealthy/v2/pkg/health"

	"github.com/Pallinder/go-randomdata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

var kubeConfigFile = os.Getenv("HOME") + "/.kube/config"

func TestClient(t *testing.T) {
	_, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreate(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	state := health.NewState()
	checkDetail := health.NewCheckDetails()
	checkDetail.AuthoritativePod = "TestCreatePod"
	checkDetail.LastRun = time.Now()
	state.CheckDetails["TestCheck"] = checkDetail
	status := NewKuberhealthyState("gotest", checkDetail)
	status.Kind = resource
	status.APIVersion = version
	result, err := client.Create(&status, resource)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", status)
	t.Logf("%+v", result)
}

func TestList(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.List(metav1.ListOptions{}, resource)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGet(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Get(metav1.GetOptions{}, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(result.Kind)
}

func TestUpdate(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	status, err := client.Get(metav1.GetOptions{}, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", status)
	randomString := randomdata.SillyName()
	checkDetail := health.NewCheckDetails()
	checkDetail.AuthoritativePod = randomString
	checkDetail.LastRun = time.Now()
	checkDetail.OK = true
	checkDetail.Errors = []string{"1", "2", "3"}
	status.Spec = checkDetail
	t.Logf("%+v", checkDetail)
	_, err = client.Update(status, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}

	result, err := client.Get(metav1.GetOptions{}, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}

	if result.Spec.AuthoritativePod != randomString {
		t.Log("Incorrect name after updating and fetching CRD.  Wanted", randomString, "but got", result.Spec.AuthoritativePod)
		t.Fail()
	}
	if result.Spec.OK != true {
		t.Log("Incorrect OK state after updating and fetching CRD.  Wanted", true, "but got", result.Spec.AuthoritativePod)
		t.Fail()
	}

	t.Log(result.Kind)
}

func TestDelete(t *testing.T) {
	client, err := Client(group, version, kubeConfigFile, defaultNamespace)
	if err != nil {
		t.Fatal(err)
	}
	state, err := client.Get(metav1.GetOptions{}, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Delete(state, resource, "gotest")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", result)
}
