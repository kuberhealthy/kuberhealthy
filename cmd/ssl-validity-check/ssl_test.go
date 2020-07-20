package main

import "testing"

func TestFetchCert(t *testing.T) {
	certs, text, err := fetchCert("google.com", "443")
	if err != nil {
		t.Fatal("Error retrieving cert:", err)
	}
	t.Log(text)
	for _, c := range certs {
		t.Log(c.DNSNames)
	}

}
