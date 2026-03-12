package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"testing"
)

func testServerTLS(t *testing.T) *tls.Config {
	cert, err := tls.LoadX509KeyPair("../../testdata/server.crt", "../../testdata/server.key")
	if err != nil {
		t.Fatal(err)
	}

	cp := x509.NewCertPool()
	rootca, err := ioutil.ReadFile("../../testdata/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	if !cp.AppendCertsFromPEM(rootca) {
		t.Fatal("ca cert err")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ServerName:   "Server",
		ClientCAs:    cp,
	}
}

func testClientTLS(t *testing.T) *tls.Config {
	cert, err := tls.LoadX509KeyPair("../../testdata/client.crt", "../../testdata/client.key")
	if err != nil {
		t.Fatal(err)
	}
	cp := x509.NewCertPool()
	rootca, err := ioutil.ReadFile("../../testdata/ca.crt")
	if err != nil {
		t.Fatal(err)
	}
	if !cp.AppendCertsFromPEM(rootca) {
		t.Fatal("ca cert err")
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ServerName:   "Server",
		RootCAs:      cp,
	}
}
