package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
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

func watchDir(d string) (chan fsNotificationChan, error) {
	log.Println("Debug: starting watcher of configmap")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		err = fmt.Errorf("error when opening watcher for: %s %w", d, err)
		return make(chan fsNotificationChan), err
	}
	defer watcher.Close()
	watchDir := make(chan fsNotificationChan)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				h := hashCreator(event.Name)
				watchDir <- fsNotificationChan{event: "configWatch: configmap has been changed at location" + event.Name, path: event.Name, failed: false, hash: h}

			case err, ok := <-watcher.Errors:
				log.Println("error: ", err)
				if err == nil {
					err = errors.New("Error return was null")
				}
				watchDir <- fsNotificationChan{event: "configWatch: failed to watch configmap directory with error: " + err.Error(), failed: true, hash: 00}
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(d)
	if err != nil {
		err = fmt.Errorf("error when adding file to watcher for: %s %w", d, err)
		return make(chan fsNotificationChan), err
	}
	return watchDir, nil
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

	fsNotificationChan, err := watchDir(configPathDir)
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
