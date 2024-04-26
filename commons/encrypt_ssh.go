package commons

import (
	"crypto/rsa"
	"encoding/pem"
	"os"
	"strings"

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

	// is pem?
	block, _ := pem.Decode(pemBytes)
	if block != nil {
		if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return nil, xerrors.Errorf("PEM blocks are encrypted")
		}

		switch block.Type {
		case "RSA PRIVATE KEY", "PRIVATE KEY", "EC PRIVATE KEY", "DSA PRIVATE KEY", "OPENSSH PRIVATE KEY":
			privateKey, err := ssh.ParseRawPrivateKey(pemBytes)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse private key file %s: %w", keyPath, err)
			}

			return privateKey, nil
		case "RSA PUBLIC KEY", "PUBLIC KEY", "EC PUBLIC KEY", "DSA PUBLIC KEY", "OPENSSH PUBLIC KEY":
			publicKey, err := ssh.ParsePublicKey(pemBytes)
			if err != nil {
				return nil, xerrors.Errorf("failed to parse public key file %s: %w", keyPath, err)
			}

			return publicKey, nil
		default:
			return nil, xerrors.Errorf("failed to parse public/private key file %s: %w", keyPath, err)
		}
	}

	// authorized key
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
