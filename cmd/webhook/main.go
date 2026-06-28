package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s-sidecar-injector/pkg/mutation"
	"k8s-sidecar-injector/pkg/webhook"
)

func main() {
	// Initialize structured logging (Go 1.21 slog)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Configuration via environment variables
	port := getEnv("WEBHOOK_PORT", "8443")
	certFile := getEnv("WEBHOOK_CERT_FILE", "/etc/webhook/certs/tls.crt")
	keyFile := getEnv("WEBHOOK_KEY_FILE", "/etc/webhook/certs/tls.key")
	autoCert := getEnv("AUTO_GENERATE_CERT", "false")
	sidecarConfigPath := getEnv("SIDECAR_CONFIG_PATH", "/etc/webhook/config/sidecar.yaml")

	slog.Info("Starting k8s-sidecar-injector",
		"port", port,
		"cert", certFile,
		"key", keyFile,
		"config", sidecarConfigPath,
	)

	// Dev Experience: Auto-TLS Fallback
	if autoCert == "true" {
		if _, err := os.Stat(certFile); os.IsNotExist(err) {
			slog.Warn("⚠️ AUTO_GENERATE_CERT enabled — not for production. Generating self-signed certificates...")
			if err := generateSelfSignedCert(certFile, keyFile); err != nil {
				slog.Error("Failed to generate self-signed certs", "error", err)
				os.Exit(1)
			}
		}
	}

	configMgr, err := mutation.NewSidecarConfigManager(sidecarConfigPath)
	if err != nil {
		slog.Error("Failed to initialize sidecar config manager", "error", err)
		os.Exit(1)
	}

	server := &webhook.Server{
		ConfigManager: configMgr,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", server.HandleMutate)
	mux.HandleFunc("/healthz", server.HandleHealthz)
	mux.HandleFunc("/readyz", server.HandleReadyz)
	mux.Handle("/metrics", server.HandleMetrics())

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Channel for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		slog.Info("Webhook server listening", "addr", srv.Addr)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			slog.Error("ListenAndServeTLS failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown or reload signal
	for {
		sig := <-stop
		if sig == syscall.SIGHUP {
			slog.Info("SIGHUP received, reloading configuration...")
			if err := configMgr.Reload(); err != nil {
				slog.Error("Failed to reload configuration", "error", err)
			}
			continue
		}
		break
	}

	slog.Info("Shutting down webhook server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Server forced to shutdown", "error", err)
	}

	slog.Info("Server exited gracefully")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func generateSelfSignedCert(certPath, keyPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"k8s-sidecar-injector-dev"},
		},
		DNSNames: []string{
			"sidecar-injector",
			"sidecar-injector.default.svc",
			"sidecar-injector.default.svc.cluster.local",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("create cert: %w", err)
	}

	certOut := &bytes.Buffer{}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	keyOut := &bytes.Buffer{}
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		return err
	}

	if err := os.WriteFile(certPath, certOut.Bytes(), 0644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyPath, keyOut.Bytes(), 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	return nil
}
