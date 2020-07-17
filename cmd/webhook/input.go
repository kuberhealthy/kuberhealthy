package main

import (
	"os"
	"strconv"

	log "github.com/sirupsen/logrus"
)

// parseDebugSettings parses debug settings and fatals on errors.
func parseDebugSettings() {
	// Enable debug logging if required.
	if len(debugEnv) != 0 {
		var err error
		debug, err = strconv.ParseBool(debugEnv)
		if err != nil {
			log.Fatalln("failed to parse DEBUG environment variable:", err)
		}
	}

	// Turn on debug logging.
	if debug {
		log.Infoln("Debug logging enabled.")
		log.SetLevel(log.DebugLevel)
	}
	log.Debugln(os.Args)
}

// parseInputValues parses all incoming environment variables for the program into globals and fatals on errors.
func parseInputValues() {
	// Parse incoming webhook types.
	// For mutating:
	if len(mutateEnv) != 0 {
		mutateEnabled, err := strconv.ParseBool(mutateEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse MUTATE:", err.Error())
		}
		mutate = mutateEnabled
		log.Infoln("Parsed MUTATE:", mutate)
	}

	// For validating:
	if len(validateEnv) != 0 {
		validateEnabled, err := strconv.ParseBool(validateEnv)
		if err != nil {
			log.Fatalln("error occurred attempting to parse VALIDATE:", err.Error())
		}
		validate = validateEnabled
		log.Infoln("Parsed VALIDATE:", validate)
	}

	// Parse incoming TLS cert location.
	certPath = defaultCertPath
	if len(certPathEnv) != 0 {
		stat, err := os.Stat(certPathEnv)
		if os.ErrPermission == err || os.ErrInvalid == err || os.ErrNotExist == err {
			log.Fatalln("unable to open certificate file at path ("+certPathEnv+"):", err.Error())
		}
		if stat.Size() == 0 {
			log.Fatalln("user given certificate file is empty:", stat.Size(), "bytes")
		}
		certPath = certPathEnv
		log.Infoln("Parsed TLS_CERT_FILE:", certPath)
	}

	// Parse incoming TLS key location.
	keyPath = defaultKeyPath
	if len(keyPathEnv) != 0 {
		stat, err := os.Stat(keyPathEnv)
		if os.ErrPermission == err || os.ErrInvalid == err || os.ErrNotExist == err {
			log.Fatalln("unable to open key file at path ("+keyPathEnv+"):", err.Error())
		}
		if stat.Size() == 0 {
			log.Fatalln("user given key file is empty:", stat.Size(), "bytes")
		}
		keyPath = keyPathEnv
		log.Infoln("Parsed TLS_KEY_FILE:", keyPath)
	}
}
