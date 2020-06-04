package main

import (
	"io/ioutil"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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

// // configChangeNotifier creates a watcher and can be used to notify of change to the configmap file on disk
// func fileChangeNotifier(file string) (chan notifyChange, error) {
// 	// log.Println("Debug: starting watcher of configmap")
// 	watcher, err := fsnotify.NewWatcher()
// 	if err != nil {
// 		err = fmt.Errorf("error when opening watcher for: %s %w", file, err)
// 		return make(chan notifyChange), err
// 	}
// 	// defer watcher.Close()

// 	notifyChan := make(chan notifyChange)

// 	go func() {
// 		for {
// 			select {
// 			case event, ok := <-watcher.Events:
// 				// ignore all events except writes to reduce spam
// 				if event.Op != fsnotify.Write {
// 					log.Debugln("event: skipped event ", event)
// 					continue
// 				}
// 				notifyChan <- notifyChange{event: "event: configmap has been changed!", path: "configmap path:" + event.Name, failed: false}
// 				if !ok {
// 					return
// 				}
// 			case err, ok := <-watcher.Errors:
// 				log.Println("error: ", err)
// 				if err == nil {
// 					err = errors.New("test")
// 				}
// 				notifyChan <- notifyChange{event: "Failed to watch file with error: " + err.Error(), failed: true, path: file}
// 				if !ok {
// 					return
// 				}
// 			}
// 		}
// 	}()

// 	err = watcher.Add(file)
// 	if err != nil {
// 		err = fmt.Errorf("error when adding file to watcher for: %s %w", file, err)
// 		return make(chan notifyChange), err
// 	}
// 	return notifyChan, nil
// }

// NotifyChange struct used for channel
type notifyChange struct {
	event  string
	path   string
	failed bool
}

// configmap watcher
func fileChangeNotifier(file string) chan notifyChange {

	notifyChan := make(chan notifyChange)

	viper.SetConfigName("kuberhealthy") // name of config file (without extension)
	viper.SetConfigType("yaml")         // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/config/") // path to look for the config file in
	// viper.AddConfigPath("$HOME/.appname") // call multiple times to add many search paths
	// viper.AddConfigPath(".")              // optionally look for config in the working directory
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
		} else {
			// Config file was found but another error was produced
		}
	}

	// Config file found and successfully parsed

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {

		// skip events that are not the write  or create operation
		if e.Op == 3 {
			log.Infoln("configReloader: event skipped beacuse it was not a write or create operation, it was a:", e.Op.String())
			return
		}
		if e.Op == 4 {
			log.Infoln("configReloader: event skipped beacuse it was not a write or create operation, it was a:", e.Op.String())
			return
		}
		if e.Op == 5 {
			log.Infoln("configReloader: event skipped beacuse it was not a write or create operation, it was a:", e.Op.String())
			return
		}
		notifyChan <- notifyChange{event: "Configmap changed with operation" + e.Op.String()}
	})
	return notifyChan
}
