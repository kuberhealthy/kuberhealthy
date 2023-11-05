package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Pallinder/go-randomdata"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/health"
)

// makeTestKuberhealthy makes a test kuberhealthy client
// that has no actual kube configuration
func makeTestKuberhealthy(t *testing.T) *Kuberhealthy {
	t.Skip()

	kh := NewKuberhealthy(&Config{})

	// override the client with a blank config
	config := &rest.Config{}
	client, _ := kubernetes.NewForConfig(config)
	kh.overrideKubeClient = client

	return kh
}

// TestWebServer tests the web server status page functionality
func TestWebServer(t *testing.T) {
	t.Skip()

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	// create a new kuberhealthy
	t.Log("Making fake check")
	kh := makeTestKuberhealthy(t)

	// add a fake check to it
	//fc := NewFakeCheck()
	t.Log("Adding fake check")
	//kh.AddCheck(fc)

	t.Log("Starting Kuberhealthy checks")
	go kh.Start(ctx)
	// give the checker time to make CRDs
	t.Log("Waiting for checks to run")
	time.Sleep(time.Second * 2)
	t.Log("Stopping Kuberhealthy checks")
	kh.StopChecks()

	// now run our test against the web server handler
	t.Log("Simulating web request")
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Error creating request", err)
	}
	err = kh.healthCheckHandler(recorder, req)
	if err != nil {
		t.Fatal("Error from health check handler:", err)
	}

	// check the http status from the server
	t.Log("Checking status code")
	if recorder.Code != http.StatusOK {
		t.Fatal("Bad response from handler", recorder.Code)
	}

	// output the response from the server
	t.Log("Reading response body")
	b, err := ioutil.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal("Error reading response body", err)
	}

	t.Log(string(b))

}

// TestWebServerNotOK tests the web server status when things are not OK
func TestWebServerNotOK(t *testing.T) {
	t.Skip()

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	// create a new kuberhealthy
	kh := makeTestKuberhealthy(t)

	// add a fake check to it with a not ok return
	//fc := NewFakeCheck()
	desiredError := randomdata.SillyName()
	//fc.Errors = []string{desiredError}
	//fc.OK = false
	//kh.AddCheck(fc)

	// run the checker for enough time to make and update CRD entries, then stop it
	go kh.Start(ctx)
	time.Sleep(time.Second * 5)
	kh.StopChecks()

	// now run our test against the web server handler
	recorder := httptest.NewRecorder()
	req, err := http.NewRequest("GET", "/", bytes.NewBufferString(""))
	if err != nil {
		t.Fatal("Error creating request", err)
	}
	err = kh.healthCheckHandler(recorder, req)
	if err != nil {
		t.Fatal("Error from health check handler:", err)
	}

	// check the http status from the server
	if recorder.Code != http.StatusOK {
		t.Fatal("Bad response from handler", recorder.Code)
	}

	// output the response from the server
	b, err := ioutil.ReadAll(recorder.Body)
	if err != nil {
		t.Fatal("Error reading response body", err)
	}
	t.Log(string(b))

	// decode the response body to validate the contents
	var state health.State
	json.Unmarshal(b, &state)

	if len(state.Errors) < 1 {
		t.Fatal("The expected error message was not set.")
	}
	if state.Errors[0] != desiredError {
		t.Fatal("The expected error message was not set. Got", state.Errors[0], "wanted", desiredError)
	}

	// check that OK is false
	if state.OK != false {
		t.Fatal("Did not observe status page failure when one was expected")
	}

}
