package main

import (
	"errors"
	"io/ioutil"
	"log"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestDefaultFsNotify(t *testing.T) {
	file := "/Users/jdowni000/Documents/Testfile"
	// check for symlinks
	if linkedPath, err := filepath.EvalSymlinks(file); err == nil && linkedPath != file {
		t.Log("symlink found for file")
		if err != nil {
			t.Log(err)
			return
		}
		file = linkedPath
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	watcher2chan := make(chan fsNotificationChan, 20)
	watcher2chan <- fsNotificationChan{event: "test"}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					t.Log("event ok is closing the channel")
					break
				}
				t.Log("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					t.Log("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if err == nil {
					err = errors.New("null error passed in")
					return
				}
				t.Log(err)
				if !ok {
					t.Log(("error channel is closing"))
					break
				}
				t.Log("error:", err)
			}
		}
	}()

	err = watcher.Add(file)
	if err != nil {
		log.Fatal(err)
	}
	time.Sleep(time.Second * 40)
}

func TestNewJD(t *testing.T) {
	log.Println("test 1")
	notifyChan, err := watchConfig2("/Users/jdowni000/Documents/Testfile")
	if err != nil {
		t.Fatal((err))
	}
	log.Println("test 2")
	notifications := <-notifyChan
	log.Println(notifications.event)
	log.Println("test 3")
	for {
		select {
		case <-notifyChan:
			log.Println("Got notifyChan message from file changing")
		case <-time.After(time.Minute * 3):
			log.Fatal("Did not see a notifyChan message for 3 minutes")
		}
	}
}

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
	notifyChan, err := watchConfig(tempFilePath)
	if err != nil {
		t.Fatal(err)
	}

	// watch for two changes to come from notifyChan
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
		case <-time.After(time.Minute * 5):
			t.Fatal("Did not see a notifyChan message for 5 seconds")
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
