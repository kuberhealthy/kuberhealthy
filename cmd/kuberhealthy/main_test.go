package main

import (
	log "github.com/sirupsen/logrus"
)

// init configures logrus before tests execute so debug output is always captured.
func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}
