package main

import (
	"context"
	"io/ioutil"
	"time"

	"github.com/codingsince1985/checksum"
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
	MaxKHJobAge               string `yaml:"maxKHJobAge,omitempty"`
	maxKHJobAge               time.Duration
	MaxCheckPodAge            string `yaml:"maxCheckPodAge,omitempty"`
	maxCheckPodAge            time.Duration
	MaxCompletedPodCount      int `yaml:"maxCompletedPodCount,omitempty"`
	MaxErrorPodCount          int `yaml:"maxErrorPodCount,omitempty"`
}

// Load loads file from disk
func (c *Config) Load(file string) error {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(b, c)
}

// watchConfig watches the target file (not directory) and notfies the supplied channel with the new md5sum
// when the content changes.  The interval supplied will be how often the file is polled.  To stop the
// watcher, close the supplied channel.
func watchConfig(ctx context.Context, filePath string, interval time.Duration) (chan string, error) {

	log.Infoln("watchConfig: Watching", filePath, "for changes")
	c := make(chan string)

	md5sum, err := hashCreator(filePath)
	if err != nil {
		return c, err
	}
	log.Infoln("watchConfig: initial hash for", filePath, "is", md5sum)

	// start watching for changes until the context ends
	go func() {
		// make a new ticker
		log.Debugln("watchConfig: starting a ticker with an interval of", interval)
		ticker := time.NewTicker(interval)

		// watch the ticker and survey the file
		for range ticker.C {
			// log.Debugln("watchConfig: starting a tick")
			// check if context is still valid and break the loop if it isnt
			select {
			case <-ctx.Done():
				ticker.Stop()
				close(c)
				log.Debugln("watchConfig: context closed. shutting down output")
				return
			default:
				// do nothing
			}

			// claculate md5sum differences
			newMD5Sum, err := hashCreator(filePath)
			if err != nil {
				log.Errorln("Error when calculating hash of:", filePath, err)
			}
			if newMD5Sum != md5sum {
				md5sum = newMD5Sum
				log.Debugln("watchConfig: sending file change notification")
				c <- md5sum
				log.Debugln("watchConfig: done sending file change notification")
			}
		}
		log.Debugln("watchConfig: shutting down")
	}()

	return c, nil
}

// configChangeLimiter takes in a channel and a maximum notification speed.  The returned channel will be notified at most
// once every specified maximum notification speed.  If the supplied channel gets no messages, then nothing will be
// sent to the returned channel.
func configChangeLimiter(maxSpeed time.Duration, inChan chan struct{}, outChan chan struct{}) {
	for range inChan {
		var messageSent bool
		for {
			// end loop when message has been sent
			if messageSent {
				log.Debugln("configChangeLimiter: change notification message sent. waiting for new change")
				break
			}

			log.Infoln("configChangeLimiter: file changed, waiting for", maxSpeed, "before sending a change notification")
			select {
			case <-time.After(maxSpeed):
				log.Infoln("configChangeLimiter: time limit has been reached:", maxSpeed, "sending message to outChan")
				outChan <- struct{}{}
				messageSent = true
			case <-inChan:
				log.Debugln("configChangeLimiter: another file change has been detected")
			}
		}
	}
	close(outChan)
}

// startConfigReloadMonitoring watches the target filepath for changes and smooths the output so
// that multiple signals do not come too rapidly.  Call the returned CancelFunc to shutdown
// all the background routines safely.
func startConfigReloadMonitoring(filePath string) (chan struct{}, context.CancelFunc, error) {
	return startConfigReloadMonitoringWithSmoothing(filePath, time.Second*2, time.Second*6)
}

func startConfigReloadMonitoringWithSmoothing(filePath string, scrapeInterval time.Duration, maxNotificationSpeed time.Duration) (chan struct{}, context.CancelFunc, error) {

	// make channels needed for limiter and spawn limiter in background
	inChan := make(chan struct{})
	outChan := make(chan struct{})

	log.Infoln("configReloader: begin monitoring of configmap change events")

	// create a context to represent this configReloader instance
	ctx, cancelFunc := context.WithCancel(context.Background())

	// begin watching the configuration file for changes in the background
	fsNotificationChan, err := watchConfig(ctx, filePath, scrapeInterval)
	if err != nil {
		return outChan, cancelFunc, err
	}

	go func() {
		for fileHash := range fsNotificationChan {
			log.Debugln("configReloader: configuration file hash has changed to:", fileHash)
			inChan <- struct{}{}
		}
		close(inChan)
		log.Debugln("configReloader: shutting down")
	}()
	go configChangeLimiter(time.Duration(maxNotificationSpeed), inChan, outChan)

	return outChan, cancelFunc, nil
}

// configReloader watchers for events in file and restarts kuberhealhty checks
func configReloader(ctx context.Context, kh *Kuberhealthy) {

	outChan, cancelFunc, err := startConfigReloadMonitoring(configPath)
	if err != nil {
		log.Errorln("configReloader: Error watching configuration file for changes:", err)
		log.Errorln("configReloader: configuration reloading disabled due to errors")
		return
	}
	defer cancelFunc()

	// when outChan gets events, reload configuration and checks
	for range outChan {
		err := cfg.Load(configPath)
		if err != nil {
			log.Errorln("configReloader: Error reloading config:", err)
			continue
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
		kh.RestartChecks(ctx)
		log.Infoln("configReloader: Kuberhealthy restarted!")
	}
	log.Infoln("configReloader: shutting down because no more signals are coming from outChan")
}

// hashcreator opens up a file and creates a hash of the file
func hashCreator(file string) (string, error) {
	return checksum.MD5sum(file)
}
