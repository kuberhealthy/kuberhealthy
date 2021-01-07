package main

import (
	"context"
	"net"
	"reflect"
	"testing"
	"time"
)

func TestCreateResolver(test *testing.T) {
	string_tol := "8.8.8.8"

	//nodeSelectors := map[string]string{}
	expectedResults := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address2 string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", string_tol+":53")
		},
	}
	//expectedResults := corev1.Toleration{}
	test.Log("testing createResolver")
	r, err := createResolver(string_tol)
	if err != nil {
		test.Errorf("%v", err)
	} else if reflect.TypeOf(*r) != reflect.TypeOf(*expectedResults) {
		test.Errorf("Expected %+v got %+v", expectedResults, r)
	}
}
