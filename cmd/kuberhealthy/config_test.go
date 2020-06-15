package main

import (
	"io/ioutil"
	"testing"
	"time"
)

func TestWatchDir(t *testing.T) {

	// create a temp directory to watch
	tempFilePath, err := ioutil.TempDir("", "kuberhealthy*")
	if err != nil {
		t.Fatal("Failed to create temp file at", tempFilePath, "with error:", err)
	}
	t.Log("created temp directory at", tempFilePath)

	// start a process that will write files into the temp directory on a delay
	t.Log("starting tempFileWriter background routine")
	go tempFileWriter(tempFilePath, t)

	// watch for changes to come back from the watchDir command
	notifyChan := watchDir(tempFilePath)
	var changeCount int
	for {
		if changeCount >= 2 {
			break
		}
		t.Log("watching for notifications from notifyChan")
		select {
		case <-notifyChan:
			changeCount++
			t.Log("Got notifyChan message from file changing")
		case <-time.After(time.Second):
			t.Fatal("Did not see a notifyChan message for one second")
		}
	}
	t.Log("two changes seen - test successful!")
}

func tempFileWriter(tempDirectory string, t *testing.T) {
	time.Sleep(time.Second / 2)
	err := ioutil.WriteFile(tempDirectory+"/writeOne.txt", []byte("this is the first test file"), 0775)
	if err != nil {
		t.Log("Failed to write temp file:", err)
	}
	t.Log("Wrote first file to temp location:", tempDirectory)

	time.Sleep(time.Second / 2)
	err = ioutil.WriteFile(tempDirectory+"/writeTwo.txt", []byte("this is the second test file"), 0775)
	if err != nil {
		t.Log("Failed to write second temp file:", err)
	}
	t.Log("Wrote second file to temp location:", tempDirectory)
}
