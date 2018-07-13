package daemonSet

import (
	"math/rand"
	"os"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz")

// randString generates a random string of specified length
func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// getHostname attempts to determine the hostname this program is running on
func getHostname() string {
	defaultHostname := "kuberhealthy"
	host, err := os.Hostname()
	if len(host) == 0 || err != nil {
		log.Warningln("Unable to determine hostname! Using default placeholder:", defaultHostname)
		return defaultHostname // default if no hostname can be found
	}
	return strings.ToLower(host)
}
