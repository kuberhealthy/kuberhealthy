package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	// Webhook server
	wh webhook

	// Environment boolean for using the validating webhook handler.
	// By default, enable validation.
	validateEnv = os.Getenv("VALIDATE")
	validate    = true

	// Environment boolean for using the mutating webhook handler
	// By default, enable mutation.
	mutateEnv = os.Getenv("MUTATE")
	mutate    = true

	// TLS cert and key pair.
	tlsPair tls.Certificate

	// User defined TLS certifcate location.
	certPathEnv = os.Getenv("TLS_CERT_FILE")
	certPath    string

	// User defined TLS key location.
	keyPathEnv = os.Getenv("TLS_KEY_FILE")
	keyPath    string

	// Interrupt signal channels.
	signalChan chan os.Signal

	// Verbose debug logging.
	debugEnv = os.Getenv("DEBUG")
	debug    bool

	// Context.
	ctx context.Context

	// Runtime scheme, codec, and deserializer.
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

const (
	// Resource names.
	khchecks = "khchecks"

	// Mutation and validation URL paths.
	mutatePath   = "/mutate"
	validatePath = "/validate"

	// Default server port.
	defaultPort = ":443"

	// Default cert and key locations.
	defaultCertPath = "/etc/certs/webhook/cert.pem"
	defaultKeyPath  = "/etc/certs/webhook/key.pem"
)

func init() {
	parseDebugSettings()

	parseInputValues()

	signalChan = make(chan os.Signal)

	log.Debugln("Loading TLS certificate and key pair.")
	tlsPair = loadTLS(keyPath, certPath)
	wh = webhook{}
	ctx = context.Background()
}

func main() {
	// Catch panics.
	var r interface{}
	defer func() {
		r = recover()
		if r != nil {
			log.Infoln("Recovered panic:", r)
		}
	}()

	// Create a request multiplexer for the webhook server.
	log.Debugln("Creating request multiplexer for kuberhealthy's dynamic admission controller.")
	mux := http.NewServeMux()
	// Turn on mutation if enabled.
	if mutate {
		log.Infoln("Mutation enabled.")
		mux.HandleFunc(mutatePath, wh.serve)
	}
	// Turn on validation if enabled.
	if validate {
		log.Infoln("Validation enabled.")
		mux.HandleFunc(validatePath, wh.serve)
	}
	wh.server = &http.Server{
		Addr:      defaultPort,
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{tlsPair}},
		Handler:   mux,
	}

	// Start the server.
	log.Infoln("Starting webhook server.")
	go startServer(&wh)
	log.Infoln("Webhook server started.")

	listenForInterrupts(ctx)
}

// listenForInterrupts watches the signal and done channels for termination.
func listenForInterrupts(ctx context.Context) {
	// Relay incoming OS interrupt signals to the signalChan.
	signal.Notify(signalChan, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)
	sig := <-signalChan
	log.Infoln("Received an interrupt signal from the signal channel. Shutting down.")
	log.Infoln("Webhook server exiting.")
	log.Debugln("Signal received was:", sig.String())

	// Shutdown the server.
	err := wh.server.Shutdown(ctx)
	if err != nil {
		log.Errorln("Unable to shutdown webhook server:", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}
