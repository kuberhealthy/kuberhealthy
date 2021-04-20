package main

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

// TestConfigReloadNotifications tests the notification of files changing with
// limiting on output duration.
// x = file write
// y = expected notification time
//       x     x            y
// |           |            |
// 0s         1s           2s
func TestConfigReloadNotificatons(t *testing.T) {

	// make temp file and write changes in the background
	tempDirectory := os.TempDir()
	testFile := tempDirectory + "testFile"
	go tempFileWriter(testFile, t)

	// write an initial blank file so the config monitor finds something without error
	err := ioutil.WriteFile(testFile, []byte("this is the second test file"), 0775)
	if err != nil {
		t.Fatal("Failed to write initial file to disk:", err)
	}

	// begin watching for changes
	t.Log("using test file:", testFile)
	outChan, cancelFunc, err := startConfigReloadMonitoringWithSmoothing(testFile, time.Second*1, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelFunc()

	// watch for expected results
	expectedNotifications := 2
	foundNotifications := 0
	startTime := time.Now()
	quickestRunTime := time.Second * 20
	quickestPossibleFinishTime := startTime.Add(quickestRunTime)
	maxRunTime := time.Second * 25
	timeout := time.After(maxRunTime)
	for {
		if foundNotifications == expectedNotifications {
			if time.Now().Before(quickestPossibleFinishTime) {
				t.Fatal("Tests ran too quickly! Duration was:", time.Now().Sub(startTime))
			}
			break
		}
		select {
		case <-outChan:
			foundNotifications++
			t.Log("Got file change notification!", foundNotifications, "/", expectedNotifications)
		case <-timeout:
			t.Fatal("Did not get expected notifications in time.")
		}
	}
}

// tempFileWriter writes files to the specified testFile on a schedule to test config file reloading
func tempFileWriter(testFile string, t *testing.T) {

	// write changes for 4 seconds every second
	for i := 0; i < 4; i++ {
		err := ioutil.WriteFile(testFile, []byte("this is the test file"+time.Now().String()), 0775)
		if err != nil {
			t.Fatal("Failed to write temp file content:", err)
		}
		t.Log("Wrote content to location:", testFile)
		time.Sleep(time.Second)
	}

	time.Sleep(time.Second * 10)

	// write changes for 4 seconds every second
	for i := 0; i < 4; i++ {
		err := ioutil.WriteFile(testFile, []byte("this is the test file"+time.Now().String()), 0775)
		if err != nil {
			t.Fatal("Failed to write temp file content:", err)
		}
		t.Log("Wrote content to location:", testFile)
		time.Sleep(time.Second)
	}
}
