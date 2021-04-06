package ssl_util

import (
	"net/url"
	"testing"
)

func TestCertPull(t *testing.T) {

	certPool, err := CreatePool()
	if err != nil {
		t.Fatal(err)
	}

	testURL, err := url.Parse("https://google.com")
	if err != nil {
		t.Fatal(err)
	}

	err = SSLHandshakeWithCertPool(testURL, certPool)
	if err != nil {
		t.Fatal(err)
	}

}
