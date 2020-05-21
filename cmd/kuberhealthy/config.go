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
	notifyChan := make(chan struct{})
	defer watcher.Close()
	// notifyChan := make(chan struct{})

	// done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// log.Infoln("event:", event)
				notifyChan <- struct{}{}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Infoln("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					log.Infoln("Failed to watch", configPath, "with error", err)
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
	notifyChan <- struct{}{}
}
