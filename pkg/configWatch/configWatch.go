// package configWatch

// import (
// 	"errors"
// 	"fmt"

// 	"github.com/fsnotify/fsnotify"
// 	log "github.com/sirupsen/logrus"
// )

// type fsNotificationChan struct {
// 	event     string
// 	path      string
// 	operation string
// }

// type configChangeChan struct {
// 	message string
// 	action  bool
// }

// func watchDir(d string) chan fsNotificationChan {
// 	log.Println("Debug: starting watcher of configmap")
// 	watcher, err := fsnotify.NewWatcher()
// 	if err != nil {
// 		err = fmt.Errorf("error when opening watcher for: %s %w", d, err)
// 		return make(chan fsNotificationChan)
// 	}
// 	defer watcher.Close()

// 	watchDir := make(chan fsNotificationChan)

// 	go func() {
// 		for {
// 			select {
// 			case event, ok := <-watcher.Events:
// 				watchDir <- fsNotificationChan{event: "configWatch: configmap has been changed at location" + event.Name, path: event.Name, operation: event.Op.String()}
// 				if !ok {
// 					return
// 				}
// 			case err, ok := <-watcher.Errors:
// 				log.Println("error: ", err)
// 				if err == nil {
// 					err = errors.New("Error return was null")
// 				}
// 				watchDir <- fsNotificationChan{event: "configWatch: failed to watch configmap directory with error: " + err.Error(), path: d, operation: ""}
// 				if !ok {
// 					return
// 				}
// 			}
// 		}
// 	}()

// 	err = watcher.Add(d)
// 	if err != nil {
// 		err = fmt.Errorf("error when adding file to watcher for: %s %w", d, err)
// 		return make(chan fsNotificationChan)
// 	}
// 	return watchDir
// }

// func configChange

// func configWatchAnalyzer(c chan fsNotificationChan) chan configChangeChan {
	
// 	configChange := make(chan configChangeChan)
// 	// Do some shit

// 		//Open a new hash interface to write to
// 		hash := md5.New()

// 		//Copy the file in the hash interface and check for any error
// 		if _, err := io.Copy(hash, c.path); err != nil {
// 			hasOrig := returnMD5String
// 		}
// 	return configChange
// }
