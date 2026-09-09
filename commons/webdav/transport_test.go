package webdav

import (
	"crypto/tls"
	"testing"
	"time"
)

func TestWebDAVTransportSecurityAndTimeouts(t *testing.T) {
	transport := newWebDAVTransport()

	if transport.TLSClientConfig == nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS10 {
		t.Fatalf("minimum TLS version = %v, want TLS 1.0", transport.TLSClientConfig.MinVersion)
	}
	if len(transport.TLSClientConfig.CipherSuites) != 25 {
		t.Errorf("configured cipher suite count = %d, want 25", len(transport.TLSClientConfig.CipherSuites))
	}
	if transport.TLSHandshakeTimeout != 10*time.Second {
		t.Errorf("TLSHandshakeTimeout = %s, want 10s", transport.TLSHandshakeTimeout)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %s, want 30s", transport.ResponseHeaderTimeout)
	}
	if transport.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %s, want 90s", transport.IdleConnTimeout)
	}
}
