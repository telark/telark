package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/telark/exporter/internal/constants"
)

const (
	rsaMinKeyBits = 2048
	noKeyUsage    = 0
)

func LoadCertBase64(certBase64 string) ([]byte, error) {
	certBytes, err := base64.StdEncoding.DecodeString(certBase64)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrCertDecodeBase64), err)
	}

	if err := validateCert(certBytes); err != nil {
		return nil, err
	}

	return certBytes, nil
}

func validateCert(certBytes []byte) error {
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return errors.New(string(constants.ErrCertInvalidPEM))
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf(string(constants.ErrCertParseFailed), err)
	}

	if err := validateExpiration(cert); err != nil {
		return err
	}

	return validateCertificateChain(cert)
}

func validateExpiration(cert *x509.Certificate) error {
	if cert.NotAfter.Before(time.Now()) {
		return fmt.Errorf(string(constants.ErrCertInvalidExpiration), cert.NotAfter)
	}
	return nil
}

func validateCertificateChain(cert *x509.Certificate) error {
	if err := validateTemporalSkew(cert); err != nil {
		return err
	}
	if err := validateCAConstraints(cert); err != nil {
		return err
	}
	if err := validatePublicKeyStrength(cert); err != nil {
		return err
	}
	return verifySelfSigned(cert)
}

func validateTemporalSkew(cert *x509.Certificate) error {
	if cert.NotBefore.After(time.Now().Add(constants.CertNotBeforeClockSkew)) {
		return fmt.Errorf(string(constants.ErrCertCertificateNotYetValid), cert.NotBefore)
	}
	return nil
}

func validateCAConstraints(cert *x509.Certificate) error {
	if !cert.IsCA {
		return nil
	}
	if !cert.BasicConstraintsValid {
		return errors.New(string(constants.ErrCertInvalidBasicConstraints))
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == noKeyUsage {
		return errors.New(string(constants.ErrCertCAKeyUsageMissing))
	}
	return nil
}

func validatePublicKeyStrength(cert *x509.Certificate) error {
	switch pk := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		if pk.N.BitLen() < rsaMinKeyBits {
			return fmt.Errorf(string(constants.ErrCertRSAKeyTooShort), pk.N.BitLen())
		}
	case *ecdsa.PublicKey:
		if pk.Curve == elliptic.P224() {
			return errors.New(string(constants.ErrCertECDSATooWeak))
		}
	default:
		return errors.New(string(constants.ErrCertUnsupportedKeyType))
	}
	return nil
}

func verifySelfSigned(cert *x509.Certificate) error {
	// verify certificate using itself as a root (typical for self-signed CA bundles)
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	verifyOpts := x509.VerifyOptions{
		Roots:       roots,
		KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
		CurrentTime: time.Now(),
	}
	if _, err := cert.Verify(verifyOpts); err != nil {
		return fmt.Errorf(string(constants.ErrCertVerificationFailed), err)
	}
	return nil
}
