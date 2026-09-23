package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/telark/exporter/internal/managers/certs"
)

const (
	certSerialNumber = 1

	// Below the 2048-bit floor the loader must reject; that rejection is the
	// assertion, so the key is deliberately weak.
	weakRSABits = 1024
)

func certB64(t *testing.T, key crypto.Signer, configure func(*x509.Certificate)) string {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(certSerialNumber),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	configure(tmpl)
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return base64.StdEncoding.EncodeToString(pemBytes)
}

func ecKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()
	k, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func asCA(c *x509.Certificate) {
	c.IsCA = true
	c.BasicConstraintsValid = true
	c.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
}

func TestLoadCertBase64Valid(t *testing.T) {
	b64 := certB64(t, ecKey(t, elliptic.P256()), asCA)
	if _, err := certs.LoadCertBase64(b64); err != nil {
		t.Errorf("valid self-signed CA rejected: %v", err)
	}
}

func TestLoadCertBase64Invalid(t *testing.T) {
	if _, err := certs.LoadCertBase64("!!!not base64!!!"); err == nil {
		t.Error("invalid base64 accepted")
	}
	if _, err := certs.LoadCertBase64(base64.StdEncoding.EncodeToString([]byte("hello"))); err == nil {
		t.Error("non-PEM payload accepted")
	}
	junkPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("junk")})
	if _, err := certs.LoadCertBase64(base64.StdEncoding.EncodeToString(junkPEM)); err == nil {
		t.Error("unparseable certificate accepted")
	}
}

func TestLoadCertBase64Expired(t *testing.T) {
	b64 := certB64(t, ecKey(t, elliptic.P256()), func(c *x509.Certificate) {
		asCA(c)
		c.NotBefore = time.Now().Add(-2 * time.Hour)
		c.NotAfter = time.Now().Add(-time.Hour)
	})
	if _, err := certs.LoadCertBase64(b64); err == nil {
		t.Error("expired certificate accepted")
	}
}

func TestLoadCertBase64NotYetValid(t *testing.T) {
	b64 := certB64(t, ecKey(t, elliptic.P256()), func(c *x509.Certificate) {
		asCA(c)
		c.NotBefore = time.Now().Add(time.Hour)
		c.NotAfter = time.Now().Add(2 * time.Hour)
	})
	if _, err := certs.LoadCertBase64(b64); err == nil {
		t.Error("not-yet-valid certificate accepted")
	}
}

func TestLoadCertBase64CAWithoutCertSign(t *testing.T) {
	b64 := certB64(t, ecKey(t, elliptic.P256()), func(c *x509.Certificate) {
		c.IsCA = true
		c.BasicConstraintsValid = true
		c.KeyUsage = x509.KeyUsageDigitalSignature // deliberately missing CertSign
	})
	if _, err := certs.LoadCertBase64(b64); err == nil {
		t.Error("CA without cert-sign usage accepted")
	}
}

func TestLoadCertBase64WeakKeys(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, weakRSABits)
	if err != nil {
		t.Fatal(err)
	}
	shortRSA := certB64(t, rsaKey, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})
	if _, err := certs.LoadCertBase64(shortRSA); err == nil {
		t.Error("RSA-1024 certificate accepted")
	}

	weakEC := certB64(t, ecKey(t, elliptic.P224()), func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})
	if _, err := certs.LoadCertBase64(weakEC); err == nil {
		t.Error("P-224 certificate accepted")
	}
}

func TestLoadCertBase64UnsupportedKey(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	b64 := certB64(t, priv, func(c *x509.Certificate) {
		c.KeyUsage = x509.KeyUsageDigitalSignature
	})
	if _, err := certs.LoadCertBase64(b64); err == nil {
		t.Error("unsupported key type accepted")
	}
}
