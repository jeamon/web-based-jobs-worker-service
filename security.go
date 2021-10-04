package main

// @security.go contains all functions related to connection security such as TLS certification
// generation and worker service security and users access authentication / authorization.

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// generateServerCertificate builds self-signed certificate and rsa-based keys then returns their pem-encoded
// format after has saved them on disk. It aborts the program if any failure during the process.
func generateServerCertificate() ([]byte, []byte) {
	log.Println("generating new self-signed server's certificate and private key")
	// ensure the presence of "certs" folder to store server's certs & key.
	createFolder(Config.HttpsServerCertsPath)
	// https://pkg.go.dev/crypto/x509#Certificate
	serverCerts := &x509.Certificate{
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		// generate a random serial number
		SerialNumber: big.NewInt(2021),
		// define the PKIX (Internet Public Key Infrastructure Using X.509).
		// fill each field with the right information based on your context.
		Subject: pkix.Name{
			Organization:  []string{"My Company"},
			Country:       []string{"My Country"},
			Province:      []string{"My City"},
			Locality:      []string{"My Locality"},
			StreetAddress: []string{"My Street Address"},
			PostalCode:    []string{"00000"},
			CommonName:    "localhost",
		},

		NotBefore: time.Now(),
		// make it valid for 1 year.
		NotAfter: time.Now().AddDate(1, 0, 0),
		// self-signed certificate.
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		// enter the right email address here.
		EmailAddresses: []string{Config.HttpsServerCertsEmail},
		// add local web server access.
		// IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		// DNSNames:    []string{"localhost"},
	}

	// dynamically fill these two fields based on worker settings.
	if Config.HttpsServerHost == "127.0.0.1" || Config.HttpsServerHost == "0.0.0.0" || Config.HttpsServerHost == "localhost" {
		serverCerts.IPAddresses = append(serverCerts.IPAddresses, net.IPv4(127, 0, 0, 1), net.IPv6loopback)
		serverCerts.DNSNames = append(serverCerts.DNSNames, "localhost")
	} else {
		// setup based on type (DNS or IP) of server address provided.
		if ip := net.ParseIP(Config.HttpsServerHost); ip != nil {
			serverCerts.IPAddresses = append(serverCerts.IPAddresses, ip)
		} else {
			serverCerts.DNSNames = append(serverCerts.DNSNames, Config.HttpsServerHost)
		}
	}

	// generate a public & private key for the certificate.
	serverPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Printf("failed to generate server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}

	log.Println("successfully created rsa-based key for server certificate.")

	// pem encode the private key.
	serverPrivKeyPEM := new(bytes.Buffer)
	err = pem.Encode(serverPrivKeyPEM, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(serverPrivKey),
	})

	if err != nil {
		log.Printf("failed to pem encode server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(2)
	}

	// dump CA private key into a file.
	if err := os.WriteFile(filepath.Join(Config.HttpsServerCertsPath, Config.HttpsServerKey), serverPrivKeyPEM.Bytes(), 0644); err != nil {
		log.Printf("failed to save on disk the server private key - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}

	log.Println("successfully pem encoded and saved server's private key.")

	// create the server certificate. https://pkg.go.dev/crypto/x509#CreateCertificate
	serverCertsBytes, err := x509.CreateCertificate(rand.Reader, serverCerts, serverCerts, &serverPrivKey.PublicKey, serverPrivKey)
	if err != nil {
		log.Printf("failed to create server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}

	log.Println("successfully created server certificate.")

	// pem encode the certificate.
	serverCertsPEM := new(bytes.Buffer)
	err = pem.Encode(serverCertsPEM, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: serverCertsBytes,
	})

	if err != nil {
		log.Printf("failed to pem encode server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(2)
	}

	// dump certificate into a file.
	if err := os.WriteFile(filepath.Join(Config.HttpsServerCertsPath, Config.HttpsServerCerts), serverCertsPEM.Bytes(), 0644); err != nil {
		log.Printf("failed to save on disk the server certificate - errmsg : %v\n", err)
		// try to remove PID file.
		os.Remove(Config.WorkerPidFilePath)
		os.Exit(1)
	}

	log.Println("successfully pem encoded and saved server's certificate.")

	return serverCertsPEM.Bytes(), serverPrivKeyPEM.Bytes()
}
