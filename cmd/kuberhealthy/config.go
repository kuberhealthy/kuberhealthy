package main

import (
	"io/ioutil"

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

// configChangeNotifier creates a watcher and can be used to notify of change to the configmap file on disk
func configChangeNotifier() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	notifyChan := make(chan notifyChange)
	defer watcher.Close()

	// done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				eventChan := event.Name
				notifyChan <- notifyChange{event: "event: configmap has been changed!", path: "configmap path:" + eventChan}
			case err, ok := <-watcher.Errors:
				if !ok {
					notifyChan <- notifyChange{failure: "Failed to watch:" + configPath}
					// log.Infoln("Failed to watch", configPath, "with error", err)
				}
				log.Infoln("error:", err)
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		log.Fatalln(err)
	}
	// <-done
	// notifyChan <- NotifyChange{event: "Monitoring of configmap complete"}
}

// NotifyChange struct used for channel
type notifyChange struct {
	event   string
	path    string
	failure string
}
