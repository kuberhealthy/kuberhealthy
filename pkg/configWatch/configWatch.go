package configWatch

import (
	"errors"
	"fmt"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

// ### top-level logic
// ```
// fsNotificationChan := watchDir(kuberhealthyConfigLocation)
// configChangeChan := configWatchAnalyzer(fsNotificationChan)
// smoothedReloadChan := smoothOutputchan(configChangeChan, time.Duration(time.Second * 20))
// for e := range smoothedReloadChan {
// 	// do KH configuration reload
// 	// set debug level
// 	// reload checks
// }
// ``
// ### Filesystem Change Notifications
// - func that uses `fsnotify` to monitor file changes on the `/etc/config` directory
//   - results get sent out via some channel
// ```

type fsNotificationChan struct {
	event     string
	path      string
	operation string
}

type configChangeChan struct {
	message string
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

	watchDir := make(chan fsNotificationChan)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				watchDir <- fsNotificationChan{event: "configWatch: configmap has been changed at location" + event.Name, path: event.Name, operation: event.Op.String()}
				if !ok {
					return
				}
			case err, ok := <-watcher.Errors:
				log.Println("error: ", err)
				if err == nil {
					err = errors.New("Error return was null")
				}
				watchDir <- fsNotificationChan{event: "configWatch: failed to watch configmap directory with error: " + err.Error(), path: d, operation: ""}
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

func configWatchAnalyzer(c chan fsNotificationChan) chan configChangeChan {
	configChange := make(chan configChangeChan)
	// Do some shit
	return configChange
}
