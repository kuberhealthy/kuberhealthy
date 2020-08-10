//Package ssl_util implements ssl check functions to kuberhealthy.
//These test TLS connectivity to a host and check current expiration
//status and time until certificate expiration.

package ssl_util

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var TimeoutSeconds = 10
var certExpired bool
var expireWarning bool
var currentTime time.Time
var hoursUntilInvalid uint
var daysUntilInvalid uint
var cert []*x509.Certificate

// CertHandshake performs a basic TLS/SSL handshake on the specified host and port, returning the cert information and any errors
func CertHandshake(host, port string) ([]*x509.Certificate, error) {
	log.Info("Testing SSL handshake on host ", host, "over port ", port)
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{&x509.Certificate{}}, "", err)
		return cert, err
	}
	defer conn.Close()

	cert = conn.ConnectionState().PeerCertificates

	err = conn.Handshake()
	if err != nil {
		log.Warn("Unable to complete SSL handshake with host", host+":", err)
		return cert, err
	} else {
		log.Println("SSL handshake to host", host, "completed successfully")
		return cert, err
	}
}

// CertExpiry checks that the cert on a given host and port is not currently expired, and that the expiration day is the specified number of days away
func CertExpiry(host, port, days string) ([]*x509.Certificate, bool, bool, uint, error) {
	log.Info("Testing SSL expiration on host ", host, "over port ", port)
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Warnln([]*x509.Certificate{&x509.Certificate{}}, "", err)
		return cert, certExpired, expireWarning, daysUntilInvalid, err
	}
	defer conn.Close()

	cert = conn.ConnectionState().PeerCertificates

	// Range over all certs found and then range over DNSNames fields. Run expiration checks only if matches to domain name from the spec file are found
	for _, c := range cert {
		for _, dom := range c.DNSNames {
			if dom == host {
				currentTime = time.Now()
				//convert # of days from pod spec from string to uint64, then to uint
				daysInt64, _ := strconv.ParseUint(days, 10, 64)
				daysInt := uint(daysInt64)

				// calculate hours until invalid to uint and convert to # of days
				hoursUntilInvalid = uint(c.NotAfter.Sub(currentTime).Hours())
				daysUntilInvalid = hoursUntilInvalid / uint(24)
				log.Println("Cert for", dom, "is valid from", c.NotBefore, "until", c.NotAfter)

				// Check that the current date/time is between the certificate's Not Before and Not After window
				if currentTime.Before(c.NotBefore) || currentTime.After(c.NotAfter) {
					certExpired = true
					log.Warn("Certificate for domain", dom, "expired on", c.NotAfter)
				}

				// Check that the # of days in the pod spec is greater than the number of days left until cert expiration
				if daysInt >= daysUntilInvalid {
					expireWarning = true
					log.Warn("Certificate for domain", dom, "will expire in", daysUntilInvalid, "days")
				}
			}
		}
	}

	return cert, certExpired, expireWarning, daysUntilInvalid, err
}
