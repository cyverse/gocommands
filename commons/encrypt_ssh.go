package commons

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	"golang.org/x/crypto/ssh"
	"golang.org/x/xerrors"
)

// GetDefaultPublicKeyPath returns default public key path, if public key does not exist, return private key path.
func GetDefaultPublicKeyPath() string {
	pubkeyPath, err := ExpandHomeDir("~/.ssh/id_rsa.pub")
	if err != nil {
		return ""
	}

	st, err := os.Stat(pubkeyPath)
	if err == nil && !st.IsDir() {
		return pubkeyPath
	}

	// not exist
	// use private key
	return GetDefaultPrivateKeyPath()
}

// GetDefaultPrivateKeyPath returns default private key path
func GetDefaultPrivateKeyPath() string {
	privkeyPath, err := ExpandHomeDir("~/.ssh/id_rsa")
	if err != nil {
		return ""
	}

	return privkeyPath
}

// DecodePublicPrivateKey decodes public or private key
func DecodePublicPrivateKey(keyPath string) (interface{}, error) {
	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, xerrors.Errorf("failed to read public/private key file %s: %w", keyPath, err)
	}

	var pemBlock *pem.Block
	isPrivateKey := false
	isPublicKey := false

	for {
		pemBlock, pemBytes = pem.Decode(pemBytes)
		if pemBlock == nil {
			break
		}

		if pemBlock.Type == "RSA PRIVATE KEY" {
			// found
			isPrivateKey = true
			break
		} else if pemBlock.Type == "RSA PUBLIC KEY" {
			isPublicKey = true
			break
		}
	}

	if pemBlock != nil {
		if isPrivateKey {
			privateKey, err := x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse PKCS1 private key: %w", err)
			}

			return privateKey, nil
		}

		if isPublicKey {
			publicKey, err := x509.ParsePKCS1PublicKey(pemBlock.Bytes)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse PKCS1 public key: %w", err)
			}

			return publicKey, nil
		}
	}

	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(pemBytes)
	if err != nil {
		return nil, xerrors.Errorf("failed to parse public key file %s: %w", keyPath, err)
	}

	parsedCryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, xerrors.Errorf("failed to get crypto public key: %w", err)
	}

	pubCrypto := parsedCryptoKey.CryptoPublicKey()
	pubKey, ok := pubCrypto.(*rsa.PublicKey)
	if !ok {
		return nil, xerrors.Errorf("failed to get RSA public key: %w", err)
	}

	return pubKey, nil
}

func DecodePrivateKey(privatekeyPath string) (*rsa.PrivateKey, error) {
	key, err := DecodePublicPrivateKey(privatekeyPath)
	if err != nil {
		return nil, err
	}

	if privKey, ok := key.(*rsa.PrivateKey); ok {
		return privKey, nil
	}

	return nil, xerrors.Errorf("failed to get private key")
}

func DecodePublicKey(publickeyPath string) (*rsa.PublicKey, error) {
	key, err := DecodePublicPrivateKey(publickeyPath)
	if err != nil {
		return nil, err
	}

	if pubKey, ok := key.(*rsa.PublicKey); ok {
		return pubKey, nil
	}

	if privKey, ok := key.(*rsa.PrivateKey); ok {
		return &privKey.PublicKey, nil
	}

	return nil, xerrors.Errorf("failed to get public key")
}
