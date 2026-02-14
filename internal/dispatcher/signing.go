// Package dispatcher provides webhook signing and mTLS support for secure dispatch.
package dispatcher

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"
)

// SigningConfig configures HMAC webhook payload signing.
type SigningConfig struct {
	// Secret is the HMAC-SHA256 signing key.
	Secret string `json:"secret" yaml:"secret"`
	// HeaderName is the HTTP header for the signature (default: X-Chronos-Signature).
	HeaderName string `json:"header_name,omitempty" yaml:"header_name,omitempty"`
	// IncludeTimestamp adds a timestamp to prevent replay attacks.
	IncludeTimestamp bool `json:"include_timestamp,omitempty" yaml:"include_timestamp,omitempty"`
	// TimestampHeaderName is the HTTP header for the timestamp (default: X-Chronos-Timestamp).
	TimestampHeaderName string `json:"timestamp_header_name,omitempty" yaml:"timestamp_header_name,omitempty"`
}

// MTLSConfig configures mutual TLS for webhook dispatch.
type MTLSConfig struct {
	// CertFile is the path to the client certificate PEM file.
	CertFile string `json:"cert_file" yaml:"cert_file"`
	// KeyFile is the path to the client private key PEM file.
	KeyFile string `json:"key_file" yaml:"key_file"`
	// CAFile is the path to the CA certificate PEM file for server verification.
	CAFile string `json:"ca_file,omitempty" yaml:"ca_file,omitempty"`
	// InsecureSkipVerify disables server certificate verification (not recommended).
	InsecureSkipVerify bool `json:"insecure_skip_verify,omitempty" yaml:"insecure_skip_verify,omitempty"`
}

const (
	defaultSignatureHeader  = "X-Chronos-Signature"
	defaultTimestampHeader  = "X-Chronos-Timestamp"
	signaturePrefix         = "sha256="
)

// ComputeHMAC computes the HMAC-SHA256 signature for the given payload and timestamp.
func ComputeHMAC(secret, payload string, timestamp int64) string {
	mac := hmac.New(sha256.New, []byte(secret))
	if timestamp > 0 {
		mac.Write([]byte(fmt.Sprintf("%d.", timestamp)))
	}
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC checks whether the provided signature matches the expected HMAC.
func VerifyHMAC(secret, payload, signature string, timestamp int64) bool {
	expected := ComputeHMAC(secret, payload, timestamp)
	return hmac.Equal([]byte(expected), []byte(signature))
}

// applyWebhookSignature signs the request body and adds signature headers.
func applyWebhookSignature(req *http.Request, cfg *SigningConfig, body string) {
	if cfg == nil || cfg.Secret == "" {
		return
	}

	sigHeader := cfg.HeaderName
	if sigHeader == "" {
		sigHeader = defaultSignatureHeader
	}

	var timestamp int64
	if cfg.IncludeTimestamp {
		timestamp = time.Now().Unix()
		tsHeader := cfg.TimestampHeaderName
		if tsHeader == "" {
			tsHeader = defaultTimestampHeader
		}
		req.Header.Set(tsHeader, strconv.FormatInt(timestamp, 10))
	}

	sig := ComputeHMAC(cfg.Secret, body, timestamp)
	req.Header.Set(sigHeader, signaturePrefix+sig)
}

// buildMTLSTransport creates an http.Transport configured with mutual TLS.
func buildMTLSTransport(cfg *MTLSConfig, base *http.Transport) (*http.Transport, error) {
	if cfg == nil {
		return base, nil
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("reading CA certificate: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	transport := base.Clone()
	transport.TLSClientConfig = tlsConfig
	return transport, nil
}
