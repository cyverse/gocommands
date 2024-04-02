package commons

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	"golang.org/x/xerrors"
)

func DecodePrivateKey(privatekeyPath string) (*rsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(privatekeyPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read private key file %s: %w", privatekeyPath, err)
	}

	var pemBlock *pem.Block

	for {
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			break
		}

		if pemBlock.Type == "RSA PRIVATE KEY" {
			// found
			break
		}
	}

	if pemBlock == nil {
		return nil, xerrors.Errorf("private key is not found in key file %s: %w", privatekeyPath, err)
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse PKCS1 private key: %w", err)
	}

	return privateKey, nil
}

func DecodePublicKey(publickeyPath string) (*rsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(publickeyPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read public key file %s: %w", publickeyPath, err)
	}

	var pemBlock *pem.Block

	for {
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			break
		}

		if pemBlock.Type == "RSA PUBLIC KEY" {
			// found
			break
		}
	}

	if pemBlock == nil {
		return nil, xerrors.Errorf("public key is not found in key file %s: %w", publickeyPath, err)
	}

	publicKey, err := x509.ParsePKCS1PublicKey(pemBlock.Bytes)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse PKCS1 public key: %w", err)
	}

	return publicKey, nil
}
