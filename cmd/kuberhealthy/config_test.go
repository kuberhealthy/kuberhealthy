package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestRenderConfig(t *testing.T) {

	// make a new default config
	cfg := Config{}
	cfg.EnableForceMaster = false
	cfg.EnableInflux = false
	cfg.ExternalCheckReportingURL = "http://localhost:8006"
	cfg.ListenAddress = "http://localhost:8006"
	cfg.LogLevel = "debug"

	// render it as yaml
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	// output the config in yaml
	fmt.Println(string(b))
}

// TestConfigReloadNotifications tests the notification of files changing with
// limiting on output duration.
// x = file write
// y = expected notification time
//
//	x     x            y
//
// |           |            |
// 0s         1s           2s
func TestConfigReloadNotificatons(t *testing.T) {

	// make temp file and write changes in the background
	tempDirectory := os.TempDir()
	testFile := tempDirectory + "testFile"

	errChan := make(chan error)
	go func(errChan chan error) {
		err := tempFileWriter(testFile, t)
		errChan <- err
	}(errChan)

	// write an initial blank file so the config monitor finds something without error
	err := os.WriteFile(testFile, []byte("this is the second test file"), 0775)
	if err != nil {
		t.Fatal("Failed to write initial file to disk:", err)
	}

	// begin watching for changes
	ctx := context.Background()
	t.Log("using test file:", testFile)
	outChan, err := startConfigReloadMonitoringWithSmoothing(ctx, testFile, time.Second*1, time.Second*5)
	if err != nil {
		t.Fatal(err)
	}

	// watch for expected results
	expectedNotifications := 2
	foundNotifications := 0
	startTime := time.Now()
	quickestRunTime := time.Second * 5
	quickestPossibleFinishTime := startTime.Add(quickestRunTime)
	maxRunTime := time.Second * 60
	timeout := time.After(maxRunTime)
	for {
		if foundNotifications == expectedNotifications {
			if time.Now().Before(quickestPossibleFinishTime) {
				t.Fatal("Tests ran too quickly! Duration was:", time.Since(startTime))
			}
			break
		}
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatal(err)
			}
		case <-outChan:
			foundNotifications++
			t.Log("Got file change notification!", foundNotifications, "/", expectedNotifications)
		case <-timeout:
			t.Fatal("Did not get expected notifications in time.")
		}
	}
}

// tempFileWriter writes files to the specified testFile on a schedule to test config file reloading
func tempFileWriter(testFile string, t *testing.T) error {

	// write changes for 4 seconds every second
	for i := 0; i < 4; i++ {
		err := os.WriteFile(testFile, []byte("this is the test file"+time.Now().String()), 0775)
		if err != nil {
			return err
		}
		t.Log("Wrote content to location:", testFile)
		time.Sleep(time.Second)
	}

	// pause for 1 second
	time.Sleep(time.Second * 1)

	// write changes for 4 seconds every second
	for i := 0; i < 4; i++ {
		err := os.WriteFile(testFile, []byte("this is the test file"+time.Now().String()), 0775)
		if err != nil {
			return err
		}
		t.Log("Wrote content to location:", testFile)
		time.Sleep(time.Second)
	}

	return nil
}
