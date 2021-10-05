package main

// @server.go contains the core function to run the embeded HTTPS server and API gateway.

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// startWebServer builds the certificates (if not present) and run the https web server.
func startWebServer(exit <-chan struct{}) error {
	// we try to load server certificate and key from disk.
	serverTLSCerts, err := tls.LoadX509KeyPair(
		filepath.Join(Config.HttpsServerCertsPath, Config.HttpsServerCerts),
		filepath.Join(Config.HttpsServerCertsPath, Config.HttpsServerKey))

	if err != nil {
		// if it fails for some reasons - we rebuild new certificate and key.
		log.Printf("failed to load server's certificate and key from disk - errmsg : %v", err)
		// rebuild self-signed certs and constructs server TLS certificate.
		serverTLSCerts, err = tls.X509KeyPair(generateServerCertificate())
		if err != nil {
			log.Printf("failed to load server's pem-encoded certificate and key - errmsg : %v\n", err)
			// try to remove PID file.
			os.Remove(Config.WorkerPidFilePath)
			os.Exit(1)
		}
	}

	// build server TLS configurations - https://pkg.go.dev/crypto/tls#Config
	tlsConfig := &tls.Config{
		// handshake with minimum TLS 1.2. MaxVersion is 1.3.
		MinVersion: tls.VersionTLS12,
		// server certificates (key with self-signed certs).
		Certificates: []tls.Certificate{serverTLSCerts},
		// elliptic curves that will be used in an ECDHE handshake, in preference order.
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.X25519, tls.CurveP256},
		// CipherSuites is a list of enabled TLS 1.0–1.2 cipher suites. The order of
		// the list is ignored. Note that TLS 1.3 ciphersuites are not configurable.
		CipherSuites: []uint16{
			// TLS v1.2 - ECDSA-based keys cipher suites.
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			// TLS v1.2 - RSA-based keys cipher suites.
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			// TLS v1.3 - some strong cipher suites.
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_AES_128_GCM_SHA256,
		},
	}

	router := http.NewServeMux()
	// initialize https web & api routes.
	if Config.EnableWebAccess {
		setupWebServerRoutes(router)
	}

	if Config.EnableAPIGateway {
		setupApiGatewayRoutes(router)
	}

	address := fmt.Sprintf("%s:%s", Config.HttpsServerHost, Config.HttpsServerPort)
	webserver := &http.Server{
		Addr:         address,
		Handler:      requestMiddleware(router),
		ErrorLog:     weblog,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    tlsConfig,
	}

	// goroutine in charge of shutting down the server when triggered.
	go func() {
		log.Println("started goroutine to shutdown web server ...")
		// wait until close or something comes in.
		<-exit
		log.Printf("shutting down the web server ... max waiting for 60 secs.")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := webserver.Shutdown(ctx); err != nil {
			// error due to closing listeners, or context timeout.
			log.Printf("failed to shutdown gracefully the web server - errmsg: %v", err)
			if err == context.DeadlineExceeded {
				log.Printf("the web server did not shutdown before 45 secs deadline.")
			} else {
				log.Printf("an error occured when closing underlying listeners.")
			}
			return
		}
		// err = nil - successfully shutdown the server.
		log.Println("the web server was successfully shutdown down.")
	}()

	// make listen on all interfaces - helps on container binding
	// if shutdown error will be http.ErrServerClosed.
	log.Printf("starting https web server and api gateway on %s ...\n", address)
	weblog.Printf("starting web server at %s\n", address)
	if err := webserver.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to start the web server on %s - errmsg: %v\n", address, err)
		return err
	}

	return nil
}
