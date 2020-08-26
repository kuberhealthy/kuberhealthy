//Package ssl_util implements ssl check functions to kuberhealthy.
//These test TLS connectivity to a host and check current expiration
//status and time until certificate expiration.

package ssl_util

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var TimeoutSeconds = 10

// CertHandshake performs a basic TLS/SSL handshake on the specified host/port, returning any errors
func CertHandshake(host, port string) error {
	log.Info("Testing SSL handshake on host ", host, " over port ", port)
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{&x509.Certificate{}}, "", err)
		return err
	}
	defer conn.Close()

	err = conn.Handshake()
	if err != nil {
		log.Warn("Unable to complete SSL handshake with host ", host+": ", err)
		return err
	}
	log.Info("SSL handshake to host ", host, " completed successfully")
	return err
}

// SelfsignHandshake takes a self-signed cert from a configmap volume, appends it to the system cert store, performs a TLS handshake on the host/port, and returns any errors
func SelfsignHandshake(host, port string) error {
	var selfsignCert = "/etc/ssl/selfsign/certificate.crt"
	log.Info("Testing SSL handshake on host ", host, " over port ", port)
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	// read the system cert pool, proceed with an empty pool if no certs found
	rootCAs, _ := x509.SystemCertPool()
	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	// read the user specified cert file; throw a fatal error if file cannot be read
	certs, err := ioutil.ReadFile(selfsignCert)
	if err != nil {
		log.Fatalf("Failed to append %q to RootCAs: %v", selfsignCert, err)
	}

	// append the user cert to the system cert pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Println("Failed to import cert ", selfsignCert, ", proceeding with default cert store")
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		RootCAs:            rootCAs,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{&x509.Certificate{}}, "", err)
		return err
	}
	defer conn.Close()

	err = conn.Handshake()
	if err != nil {
		log.Warn("Unable to complete SSL handshake with host ", host+": ", err)
		return err
	}
	log.Info("SSL handshake to host ", host, " completed successfully")
	return err
}

// CertExpiry returns bool values indicating if the cert on a given host and port are currently exiring or if the expiration is the specified number of days away, and any errors
func CertExpiry(host, port, days string, overrideTLS bool) (bool, bool, error) {
	log.Info("Testing SSL expiration on host ", host, " over port ", port)
	var certExpired bool
	var expireWarning bool

	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	// InsecureSkipVerify should be false except for testing purposes or checking a self-signed certificate
	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: overrideTLS,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{&x509.Certificate{}}, "", err)
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
