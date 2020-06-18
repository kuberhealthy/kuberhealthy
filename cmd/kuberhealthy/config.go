package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// Config holds all configurable options
type Config struct {
	kubeConfigFile            string
	ListenAddress             string `yaml:"listenAddress,omitempty"`
	EnableForceMaster         bool   `yaml:"enableForceMaster,omitempty"`
	LogLevel                  string `yaml:"logLevel,omitempty"`
	InfluxUsername            string `yaml:"influxUsername,omitempty"`
	InfluxPassword            string `yaml:"influxPassword,omitempty"`
	InfluxURL                 string `yaml:"influxURL,omitempty"`
	InfluxDB                  string `yaml:"influxDB,omitempty"`
	EnableInflux              bool   `yaml:"enableInflux,omitempty"`
	ExternalCheckReportingURL string `yaml:"externalCheckReportingURL,omitempty"`
}

// Load loads file from disk
func (c *Config) Load(file string) error {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, c)
}

type fsNotificationChan struct {
	event  string
	path   string
	failed bool
	hash   int64
}

type configChangeChan struct {
	message string
	path    string
	action  bool
}

type actionNeededChan struct {
	action bool
}

func watchConfig(locations ...string) (chan fsNotificationChan, error) {

	log.Println("configWatch: starting watcher of the following locations:", locations)
	watchEventsChan := make(chan fsNotificationChan, 20)

	// create new watcher with fsnotify
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		err = fmt.Errorf("confnigWatch: error when opening watcher for: %s %w", locations, err)
		return make(chan fsNotificationChan), err
	}
	defer watcher.Close()

	for _, location := range locations {
		// evaluating if file is a symblink and sets file to symlink to be watched
		if linkedPath, err := filepath.EvalSymlinks(location); err == nil && linkedPath != location {
			if err != nil {
				log.Errorln("configWatch:", err)
				return watchEventsChan, err
			}
			location = linkedPath
		}

		err = watcher.Add(location)
		log.Debugln("configWatch: starting watch on file: ", location)
		if err != nil {
			err = fmt.Errorf("configWatch: error when adding file to watcher for: %s %w", location, err)
			return make(chan fsNotificationChan), err
		}
	}

	// launch go routine to handle fsnotify events
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				log.Debugln("configWatch: saw an event from fsnotify")
				if !ok {
					log.Debugln("configWatch: event channel closed. returning")
					return
				}
				h := hashCreator(event.Name)
				watchEventsChan <- fsNotificationChan{event: "configWatch: configmap has been changed at location" + event.Name, path: event.Name, failed: false, hash: h}

			case err, ok := <-watcher.Errors:
				log.Warningln("configWatch: error: ", err)
				if err == nil {
					err = errors.New("Error return was null")
				}
				watchEventsChan <- fsNotificationChan{event: "configWatch: failed to watch configmap directory with error: " + err.Error(), failed: true, hash: 00}
				if !ok {
					log.Debugln("configWatch: error channel closed. returning")
					return
				}
			}
		}
	}()
	return watchEventsChan, nil
}

func watchConfig2(file string) (chan fsNotificationChan, error) {
	log.Println("Debug: starting watcher of configmap")

	// check for symlinks
	if linkedPath, err := filepath.EvalSymlinks(file); err == nil && linkedPath != file {
		log.Debugln("symlink found for file")
		if err != nil {
			log.Errorln(err)
			return make(chan fsNotificationChan, 20), err
		}
		file = linkedPath
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(file)
	log.Debugln("configWatch: starting watch on file: ", file)
	if err != nil {
		log.Fatal(err)
	}

	watcher2chan := make(chan fsNotificationChan, 20)

	go func() {
		for {
			log.Debugln("I am in the go routine")
			select {
			case event, ok := <-watcher.Events:
				log.Debugln("event seen in go routine at least")
				if !ok {
					log.Debugln("watcher events ok is shutting down")
					return
				}
				watcher2chan <- fsNotificationChan{event: "configWatch2:" + event.Name}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if err == nil {
					err = errors.New("null error passed in")
				}
				log.Println("error being passed :", err)
				if !ok {
					log.Debugln("watcher error is shutting down")

					return
				}
				watcher2chan <- fsNotificationChan{event: "configWatch2: error channel:" + err.Error()}
			}
		}
	}()

	return watcher2chan, nil
}

// configWatchAnalyzer watchers for events of configmap chnages and compares the known hash to the known for chnages to determine if kuberhealthy restart is required
func configWatchAnalyzer(c chan fsNotificationChan) chan configChangeChan {

	configChange := make(chan configChangeChan)
	currentHash := hashCreator(configPath)
	notify := <-c
	// Do some shit
	go func() {
		if notify.failed == true {
			configChange <- configChangeChan{message: notify.event, path: notify.path, action: false}
		}
		if currentHash != notify.hash {
			configChange <- configChangeChan{message: "configmap change event did not change the file configureations at location:" + notify.path, path: notify.path, action: false}
		}
		configChange <- configChangeChan{message: notify.event, path: notify.path, action: true}
		currentHash = notify.hash

	}()
	return configChange
}

func smoothedOutput(maxSpeed time.Duration, c chan configChangeChan) chan actionNeededChan {
	msg := <-c
	action := make(chan actionNeededChan)

	log.Infoln(msg.message)
	for range c {
		for {
			log.Infoln("configmap changed, waiting to receive another change or proceeding to reload checks after", maxSpeed)
			select {
			case <-time.After(maxSpeed):
				log.Infoln("time limit has been reached:", maxSpeed, "requesting kuberhealthy restart")
				action <- actionNeededChan{action: true}
			case <-c:
				log.Infoln("another configmap change has been detected, waiting an addition", maxSpeed, "before requesting a kuberhealthy restart")
				action <- actionNeededChan{action: false}
			}
		}

	}
	return action
}

// configReloader watchers for events in file and restarts kuberhealhty checks
func configReloader(kh *Kuberhealthy) {

	fsNotificationChan, err := watchConfig(configPathDir)
	if err != nil {
		log.Errorln("configReloader: Error watching config directory:", err)
		log.Errorln("configReloader: configuration reloading disabled due to errors")
		return
	}

	configChangeChan := configWatchAnalyzer(fsNotificationChan)
	smoothedReloadChan := smoothedOutput(time.Duration(time.Second*20), configChangeChan)
	reload := <-smoothedReloadChan
	if reload.action {
		err := cfg.Load(configPath)
		if err != nil {
			log.Errorln("configReloader: Error reloading config:", err)
		}

		// reparse and set logging level
		parsedLogLevel, err := log.ParseLevel(cfg.LogLevel)
		if err != nil {
			log.Warningln("Unable to parse log-level flag: ", err)
		} else {
			log.Infoln("Setting log level to:", parsedLogLevel)
			log.SetLevel(parsedLogLevel)
		}

		// reload checks
		kh.RestartChecks()
		log.Infoln("configReloader: Kuberhealthy restarted!")
	}
	log.Infoln("configReloader: XXXX")
}

// hashcreator opens up a file and creates a hash of the file
func hashCreator(file string) int64 {
	f, err := os.Open(file)
	if err != nil {
		log.Infoln(err)
	}
	defer f.Close()

	//Open a new hash interface to write to
	hash := md5.New()

	h, err := io.Copy(hash, f)
	if err != nil {
		log.Infoln(err)
	}
	return h
}