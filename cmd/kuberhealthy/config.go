package main

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/fsnotify/fsnotify"
	"github.com/jivesearch/jivesearch/log"
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
func fileChangeNotifier(file string) (chan notifyChange, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		err = fmt.Errorf("error when opening watcher for: %s %w", file, err)
		return make(chan notifyChange), err
	}
	defer watcher.Close()

	notifyChan := make(chan notifyChange)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				// ignore all events except writes to reduce spam
				if event.Op != fsnotify.Write {
					log.Debugln("event: skipped event ", event)
					continue
				}
				notifyChan <- notifyChange{event: "event: configmap has been changed!", path: "configmap path:" + event.Name, failed: false}
				if !ok {
					return
				}
			case err, ok := <-watcher.Errors:
				if err == nil {
					err = errors.New("")
				}
				notifyChan <- notifyChange{event: "Failed to watch file with error: " + err.Error(), failed: true, path: file}
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(file)
	if err != nil {
		err = fmt.Errorf("error when adding file to watcher for: %s %w", file, err)
		return make(chan notifyChange), err
	}
	return notifyChan, nil
}

// NotifyChange struct used for channel
type notifyChange struct {
	event  string
	path   string
	failed bool
}
