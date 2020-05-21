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
	ListenAddress             string `yaml:"listenAddress"`
	EnableForceMaster         bool   `yaml:"enableForceMaster"`
	LogLevel                  string `yaml:"logLevel"`
	InfluxUsername            string `yaml:"influxUsername"`
	InfluxPassword            string `yaml:"influxPassword"`
	InfluxURL                 string `yaml:"influxURL"`
	InfluxDB                  string `yaml:"influxDB"`
	EnableInflux              bool   `yaml:"enableInflux"`
	ExternalCheckReportingURL string `yaml:"externalCheckReportingURL"`
}

// Load loads file from disk
func (c *Config) Load(file string) error {

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, c)
}

// configMonitor creates a watcher and can be used to notify of change to configmap
func configMonitor() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalln(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Infoln("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Infoln("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Infoln("error:", err)
			}
		}
	}()

	err = watcher.Add(configPath)
	if err != nil {
		log.Fatalln(err)
	}
	<-done
}
