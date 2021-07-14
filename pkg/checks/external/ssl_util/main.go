//Package ssl_util implements ssl check functions to kuberhealthy.
//These test TLS connectivity to a host and check current expiration
//status and time until certificate expiration.

package ssl_util

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var TimeoutSeconds = 10

const kubernetesCAFileLocation = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
const selfSignedCertLocation = "/etc/ssl/selfsign/certificate.crt"

// kubernetesCAPresent returns 'true' if the included Kubernetes CA is available
func KubernetesCAPresent() bool {
	return filePresent(kubernetesCAFileLocation)
}

// SelfSignedCAPresent determines if the user has uploaded a custom CA to use for certificate validation
func SelfSignedCAPresent() bool {
	return filePresent(selfSignedCertLocation)
}

// filePresent returns 'true' if the specified file exists
func filePresent(filePath string) bool {
	if _, err := os.Stat(filePath); err == nil {
		return true
	}
	return false
}

// certPoolFromFile creates a cert pool from the specified CA file and returns it
func certPoolFromFile(filePath string) (*x509.CertPool, error) {

	// read the file bytes from disk
	b, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// make a new cert pool and append certs from file
	certPool := x509.NewCertPool()
	ok := certPool.AppendCertsFromPEM(b)
	if !ok {
		return nil, fmt.Errorf("error parsing certs from file %s", filePath)
	}

	// append the user cert to the system cert pool
	log.Info("Certificate file successfully appended to cert pool")

	return certPool, err
}

// FetchKubernetesSelfSignedCertFromDisk fetches the kubernetes self-signed cert placed on disk within pods as an
// *x509.Certificate.
func FetchKubernetesSelfSignedCertFromDisk() ([]byte, error) {
	certs, err := ioutil.ReadFile(kubernetesCAFileLocation)
	if err != nil {
		return nil, fmt.Errorf("error reading kubernetes certificate authority file: %w", err)
	}

	return certs, nil
}

// fetchSelfSignedCertFromDisk fetches the self-signed cert placed on disk within pods as an
// *x509.Certificate.
func fetchSelfSignedCertFromDisk() ([]byte, error) {
	certs, err := ioutil.ReadFile(selfSignedCertLocation)
	if err != nil {
		return nil, fmt.Errorf("error reading custom certificate file: %w", err)
	}

	return certs, nil
}

// AppendKubernetesCertsToPool appends the kubernetes certificates on disk (in the pod) to the
// supplied cert pool.
func AppendKubernetesCertsToPool(pool *x509.CertPool) error {
	certData, err := FetchKubernetesSelfSignedCertFromDisk()
	if err != nil {
		return fmt.Errorf("error fetching cert data from disk: %w", err)
	}
	ok := pool.AppendCertsFromPEM(certData)
	if !ok {
		log.Warningln("failed to append cert to pem when appending kubernetes certs to cert pool")
	}
	return nil
}

// CreatePool creates a cert pool depending on if a Kubernetes CA is found or a custom CA cert is mounted
// at /etc/ssl/selfsign/certificate.crt
func CreatePool() (*x509.CertPool, error) {
	if SelfSignedCAPresent() {
		log.Infoln("Using self signed CA mounted from ", selfSignedCertLocation)
		return certPoolFromFile(selfSignedCertLocation)
	}

	log.Infoln("Using default certs plus Kubernetes cluster CA mounted from ", kubernetesCAFileLocation)
	defaultPool, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}

	// append kubernetes certs to system default certs
	log.Infoln("Appending Kubernetes SSL certificate authority to cert pool...")
	err = AppendKubernetesCertsToPool(defaultPool)
	if err != nil {
		return nil, err
	}

	return defaultPool, nil
}

// SSLHandshakeWithCertPool does an SSL handshake with the specified cert pool instead of
// the default system certificate pool
func SSLHandshakeWithCertPool(url *url.URL, certPool *x509.CertPool) error {

	// ensure an https url was passed
	if url.Scheme != "https" {
		return fmt.Errorf("error doing SSL handshake.  The url specified %s was not an https URL", url.String())
	}

	// create our dialer
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	// dial to the TCP endpoint
	conn, err := tls.DialWithDialer(d, "tcp", url.Hostname()+":"+url.Port(), &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		RootCAs:            certPool,
	})
	if err != nil {
		return fmt.Errorf("error making connection to perform TLS handshake: %w", err)
	}
	defer conn.Close()

	// do the SSL handshake
	err = conn.Handshake()
	if err != nil {
		return fmt.Errorf("unable to perform TLS handshake: %w", err)
	}

	return nil
}

// SSLHandshake does an https handshake and returns any errors encountered
func SSLHandshake(siteURL *url.URL) error {

	certPool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}

	return SSLHandshakeWithCertPool(siteURL, certPool)

}

// CertExpiry returns bool values indicating if the cert on a given host and port are currently exiring or if the expiration is the specified number of days away, and any errors
func CertExpiry(host, port, days string, overrideTLS bool) (bool, bool, error) {
	log.Info("Testing SSL expiration on host ", host, " over port ", port)
	var certExpired bool
	var expireWarning bool

	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	// InsecureSkipVerify should be false unless checking a self-signed certificate
	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: overrideTLS,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{}, "", err)
		return certExpired, expireWarning, err
	}
	defer conn.Close()

	// var cert is assigned the slice of certs that can be pulled from a given host
	cert := conn.ConnectionState().PeerCertificates
	currentTime := time.Now()

	// convert # of days declared in pod spec from string to uint64, then to uint, to compare against cert expiration info
	daysInt64, _ := strconv.ParseUint(days, 10, 64)
	daysInt := uint(daysInt64)

	// calculate # of hours until the domain cert (cert[0] from the slice) is invalid, then convery to uint and to # of days
	daysUntilInvalid := (uint(cert[0].NotAfter.Sub(currentTime).Hours())) / uint(24)
	log.Info("Certificate for ", host, " is valid from ", cert[0].NotBefore, " until ", cert[0].NotAfter)

	// check that the current date/time is between the certificate's Not Before and Not After window
	if currentTime.Before(cert[0].NotBefore) || currentTime.After(cert[0].NotAfter) {
		certExpired = true
		log.Warn("Certificate for domain ", host, " expired on ", cert[0].NotAfter)
	}

	// check that the # of days in the pod spec is greater than the number of days left until cert expiration
	if daysInt >= daysUntilInvalid {
		expireWarning = true
		log.Warn("Certificate for domain ", host, " will expire in ", daysUntilInvalid, " days")
	}

	if (daysInt <= daysUntilInvalid) && (currentTime.Before(cert[0].NotAfter) || currentTime.After(cert[0].NotBefore)) {
		log.Info("Certificate for domain ", host, " is currently valid and will expire in ", daysUntilInvalid, " days")
	}
	return certExpired, expireWarning, err
}
