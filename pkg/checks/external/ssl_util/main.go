//Package ssl_util implements ssl check functions to kuberhealthy.
//These test TLS connectivity to a host and check current expiration
//status and time until certificate expiration.

package ssl_util

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var TimeoutSeconds = 10

// fileUploaded returns 'true' if the pem formatted cert has been mounted at /etc/ssl/selfsign/certificate.crt
func fileUploaded() bool {
	var certUploaded bool
	if _, err := os.Stat("/etc/ssl/selfsign/certificate.crt"); err == nil {
		log.Info("Certificate file uploaded from spec")
		certUploaded = true
	}
	return certUploaded
}

// certFromHost makes an insecure connection to the host specified and returns the SSL cert that matches the hostname
func certFromHost(host, port string) (*x509.Certificate, error) {
	var hostCert *x509.Certificate
	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	// Connect insecurely to the host, range through all certificates found, find the cert that matches the host name for the check, and return it
	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	})

	if err != nil {
		log.Error("Error retrieving host certificate: ", []*x509.Certificate{&x509.Certificate{}}, "", err)
		return hostCert, err
	}

	defer conn.Close()
	cert := conn.ConnectionState().PeerCertificates

	for _, clientCert := range cert {
		for _, certDNS := range clientCert.DNSNames {
			if certDNS == host {
				hostCert = clientCert
			}
		}
	}

	if hostCert == nil {
		err = errors.New("Empty certificate returned")
	}
	return hostCert, err
}

// poolFromHost checks for a pem certificate mounted by the check config map, and if one is not present it creates a new cert pool using the CertFromHost function
func poolFromHost(host, port string) (*x509.CertPool, error) {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		log.Warn("Unable to retrieve system certificate pool: ", err)
	}

	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	hostCert, err := certFromHost(host, port)
	if hostCert == nil {
		err = errors.New("Empty certificate returned")
	}

	if hostCert != nil {
		rootCAs.AddCert(hostCert)
		log.Info("Certificate from host successfully appended to cert pool")
	}
	return rootCAs, err
}

// poolFromFile checks for a pem certificate mounted by the check config map, and appends it to the system cert pool
func poolFromFile(host, port string) (*x509.CertPool, error) {
	var selfsignCert string
	rootCAs, err := x509.SystemCertPool()

	if rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}

	selfsignCert = "/etc/ssl/selfsign/certificate.crt"
	certs, err := ioutil.ReadFile(selfsignCert)
	if err != nil {
		log.Error("Error reading certificate file: ", selfsignCert, err)
	}

	// append the user cert to the system cert pool
	if ok := rootCAs.AppendCertsFromPEM(certs); !ok {
		log.Error("Failed to import cert from file: ", selfsignCert)
		return rootCAs, err
	}
	log.Info("Certificate file successfully appended to cert pool")

	return rootCAs, err
}

// selfSignPool determines if an SSL cert has been provided via configmap, and returns an error and a certificate map to check against
func selfSignPool(host, port string) (*x509.CertPool, error) {
	certProvided := fileUploaded()
	if certProvided {
		certPool, err := poolFromFile(host, port)
		return certPool, err
	}
	certPool, err := poolFromHost(host, port)

	return certPool, err
}

// SSLHandshake imports a certificate pool, a
func CertHandshake(host, port string, selfSigned bool) error {
	var certPool *x509.CertPool
	var err error

	// Check if the check is being run for a self-signed cert. If so, check to see if the file has been uploaded
	if selfSigned {
		certPool, err = selfSignPool(host, port)
		if err != nil {
			log.Error("Error generating self-signed certificate pool: ", err)
			return err
		}
	}

	// Generaete a default system cert pool if the check is not for a self-signed certificate
	if !selfSigned {
		certPool, err = x509.SystemCertPool()
		if err != nil {
			log.Warn("Unable to retrieve system certificate pool: ", err)
		}
	}

	d := &net.Dialer{
		Timeout: time.Duration(TimeoutSeconds) * time.Second,
	}

	conn, err := tls.DialWithDialer(d, "tcp", host+":"+port, &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
		RootCAs:            certPool,
	})

	if err != nil {
		log.Error("Error performing TLS handshake: ", []*x509.Certificate{&x509.Certificate{}}, "", err)
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

	// InsecureSkipVerify should be false unless checking a self-signed certificate
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
