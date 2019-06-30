package main

import (
	"testing"

	"github.com/Comcast/kuberhealthy/pkg/checks/external"
	"github.com/Comcast/kuberhealthy/pkg/kubeClient"

	log "github.com/sirupsen/logrus"
)

func init(){
	// tests always run with debug logging
	log.SetLevel(log.DebugLevel)
}

