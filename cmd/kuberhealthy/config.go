package main

import (
	"context"
	"os"
	"time"

	"github.com/codingsince1985/checksum"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config holds all configurable options
type Config struct {
	kubeConfigFile            string                    `yaml:"kubeConfigFile"`
	ListenAddress             string                    `yaml:"listenAddress"`
	EnableForceMaster         bool                      `yaml:"enableForceMaster"`
	LogLevel                  string                    `yaml:"logLevel"`
	InfluxUsername            string                    `yaml:"influxUsername"`
	InfluxPassword            string                    `yaml:"influxPassword"`
	InfluxURL                 string                    `yaml:"influxURL"`
	InfluxDB                  string                    `yaml:"influxDB"`
	EnableInflux              bool                      `yaml:"enableInflux"`
	ExternalCheckReportingURL string                    `yaml:"externalCheckReportingURL"`
	MaxKHJobAge               time.Duration             `yaml:"maxKHJobAge"`
	MaxCheckPodAge            time.Duration             `yaml:"maxCheckPodAge"`
	MaxCompletedPodCount      int                       `yaml:"maxCompletedPodCount"`
	MaxErrorPodCount          int                       `yaml:"maxErrorPodCount"`
	StateMetadata             map[string]string         `yaml:"stateMetadata,omitempty"`
	PromMetricsConfig         metrics.PromMetricsConfig `yaml:"promMetricsConfig,omitempty"`
	TargetNamespace           string                    `yaml:"namespace"` // TargetNamespace sets the namespace that Kuberhealthy will operate in.  By default, this is blank, which means
	// all namespaces.  However, for multi-tennant environments you may wish to set this.
}

// Load loads file from disk
func (c *Config) Load(file string) error {
	b, err := os.ReadFile(file)
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
	c := make(chan string, 1)

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
				select {
				case c <- md5sum:
					log.Debugln("watchConfig: queued reload notification")
				default:
					log.Debugln("watchConfig: skipping reload notification because config reload already queued")
				}
				log.Debugln("watchConfig: done sending file change notification")
			}
		}
		log.Debugln("watchConfig: shutting down")
	}()

	return c, nil
}

// startConfigReloadMonitoring watches the target filepath for changes and smooths the output so
// that multiple signals do not come too rapidly.  Call the returned CancelFunc to shutdown
// all the background routines safely.
func startConfigReloadMonitoring(ctx context.Context, filePath string) (chan struct{}, error) {
	return startConfigReloadMonitoringWithSmoothing(ctx, filePath, time.Second*2, time.Second*6)
}

func startConfigReloadMonitoringWithSmoothing(ctx context.Context, filePath string, scrapeInterval time.Duration, maxNotificationSpeed time.Duration) (chan struct{}, error) {

	// make channels needed for limiter and spawn limiter in background
	outChan := make(chan struct{})

	log.Infoln("configReloader: begin monitoring of configmap change events")

	// begin watching the configuration file for changes in the background
	fsNotificationChan, err := watchConfig(ctx, filePath, scrapeInterval)
	if err != nil {
		return outChan, err
	}

	// spawn a go routine to watch for notifications and send them every interval
	go func(ctx context.Context, fsNotificationChan chan string) {
		for {
			time.Sleep(maxNotificationSpeed) // sleep the maximum notifciation time before sending
			select {
			case <-ctx.Done(): // end when context killed
				log.Debugln("configReloader: shutting down")
				return
			case <-fsNotificationChan:
				outChan <- struct{}{}
				log.Debugln("configReloader: configuration file hash has changed")
			default:
				log.Debugln("configReloader: no configuration reload this tick")
			}
		}
	}(ctx, fsNotificationChan)

	return outChan, nil
}

// configReloadNotifier watchers for events in file, reloads the configuration, and notifies upstream to restart checks
func configReloadNotifier(ctx context.Context, notifyChan chan struct{}) {

	outChan, err := startConfigReloadMonitoring(ctx, configPath)
	if err != nil {
		log.Errorln("configReloader: Error watching configuration file for changes:", err)
		log.Errorln("configReloader: configuration reloading disabled due to errors")
		return
	}

	// when outChan gets events, reload configuration and checks
	for range outChan {
		// if the context has expired, then shut down the config reload notifier entirely
		select {
		case <-ctx.Done():
			log.Debugln("configReloader: stopped notifying config reloads due to context cancellation")
			return
		default:
		}

		log.Debugln("configReloader: loading new configuration")

		// setup config
		err := setUpConfig()
		if err != nil {
			log.Errorln("configReloader: Error reloading and setting up config:", err)
			continue
		}
		log.Debugln("configReloader: loaded new configuration:", cfg)

		// reparse and set logging level
		parsedLogLevel, err := log.ParseLevel(cfg.LogLevel)
		if err != nil {
			log.Warningln("Unable to parse log-level flag: ", err)
		} else {
			log.Infoln("Setting log level to:", parsedLogLevel)
			log.SetLevel(parsedLogLevel)
		}
		notifyChan <- struct{}{}
	}
	log.Infoln("configReloader: shutting down because no more signals are coming from outChan")
}

// hashcreator opens up a file and creates a hash of the file
func hashCreator(file string) (string, error) {
	return checksum.MD5sum(file)
}
