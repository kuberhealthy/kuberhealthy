package main

import "testing"

func TestFetchCert(t *testing.T) {
	testDomainName := "google.com"
	defaultPortNum := "443"

	certs, text, err := FetchCert(testDomainName, defaultPortNum)
	if err != nil {
		t.Fatal("Error retrieving cert:", err)
	}

	if len(certs) == 0 {
		t.Fatal("No certificates found on domain name", testDomainName, "port number:", defaultPortNum)
	}

	t.Log(text)
	for _, c := range certs {
		t.Log(c.DNSNames)
		t.Log(c.NotBefore, c.NotAfter)
	}

}

/*
	Options next:
		-Two functions
			-test-cert-handshake
				-use conn handshake to confirm connection can be made with SSL
			-test-cert-expiration


test-cert-expiration
test-cert-handshake

Handshake  https://golang.org/pkg/crypto/tls/#Conn.Handshake
	hand shake requires tls.Conn: https://golang.org/pkg/crypto/tls/#Conn
		func Client returns tls.Conn: https://golang.org/pkg/net/#Conn
			func Client(conn net.Conn, config *Config) *Conn

net.Dial returns Conn
*/
