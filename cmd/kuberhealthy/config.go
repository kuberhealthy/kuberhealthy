package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

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

// NotifyChange struct used for channel
// type notifyChange struct {
// 	event  string
// 	path   string
// 	failed bool
// }

// // configChangeNotifier creates a watcher and can be used to notify of change to the configmap file on disk
// func fileChangeNotifier(file string) chan notifyChange {

// 	notifyChan := make(chan notifyChange)

// 	viper.SetConfigName("kuberhealthy") // name of config file (without extension)
// 	viper.SetConfigType("yaml")         // REQUIRED if the config file does not have the extension in the name
// 	viper.AddConfigPath("/etc/config/") // path to look for the config file in
// 	// viper.AddConfigPath("$HOME/.appname") // call multiple times to add many search paths
// 	// viper.AddConfigPath(".")              // optionally look for config in the working directory
// 	if err := viper.ReadInConfig(); err != nil {
// 		if err != nil { // Handle errors reading the config file
// 			log.Infoln("configmap file ERROR!")
// 		}
// 	}

// 	// Config file found and successfully parsed

// 	viper.WatchConfig()
// 	viper.OnConfigChange(func(e fsnotify.Event) {

// 		// skip events that are not the write  or create operation
// 		if e.Op == 3 || e.Op == 4 || e.Op == 5 {
// 			log.Infoln("configReloader: event skipped beacuse it was not a write or create operation, it was a:", e.Op.String())
// 			return
// 		}
// 	})
// 	return notifyChan
// }

type fsNotificationChan struct {
	event string
	path  string
	hash  int64
}

type configChangeChan struct {
	message string
	path    string
	action  bool
}

func watchDir(d string) chan fsNotificationChan {
	log.Println("Debug: starting watcher of configmap")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		err = fmt.Errorf("error when opening watcher for: %s %w", d, err)
		return make(chan fsNotificationChan)
	}
	defer watcher.Close()
	{
	}
	watchDir := make(chan fsNotificationChan)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				h := hashCreator(event.Name)
				watchDir <- fsNotificationChan{event: "configWatch: configmap has been changed at location" + event.Name, path: event.Name, hash: h}

			case err, ok := <-watcher.Errors:
				log.Println("error: ", err)
				if err == nil {
					err = errors.New("Error return was null")
				}
				watchDir <- fsNotificationChan{event: "configWatch: failed to watch configmap directory with error: " + err.Error(), hash: 00}
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(d)
	if err != nil {
		err = fmt.Errorf("error when adding file to watcher for: %s %w", d, err)
		return make(chan fsNotificationChan)
	}
	return watchDir
}

// configWatchAnalyzer watchers for events of configmap chnages and compares the known hash to the known for chnages to determine if kuberhealthy restart is required
func configWatchAnalyzer(c chan fsNotificationChan) chan configChangeChan {

	configChange := make(chan configChangeChan)
	currentHash := hashCreator(configPath)
	notify := <-c
	// Do some shit
	go func() {
		if currentHash == notify.hash {
			configChange <- configChangeChan{message: notify.event, path: notify.path, action: true}
		}
		configChange <- configChangeChan{message: "configmap change event did not change the file at location:" + notify.path, path: notify.path, action: false}
	}()
	return configChange
}

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
