package main

import (
	log "github.com/sirupsen/logrus"
)

func init() {
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}
