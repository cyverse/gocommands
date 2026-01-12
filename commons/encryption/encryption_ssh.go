package encryption

import (
	"crypto/rsa"
	"encoding/pem"
	"os"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/commons/path"
	"golang.org/x/crypto/ssh"
)

const (
	defaultPublicKeyPath  = "~/.ssh/id_rsa.pub"
	defaultPrivateKeyPath = "~/.ssh/id_rsa"
)

// GetDefaultPublicKeyPath returns default public key path, if public key does not exist, return private key path.
func GetDefaultPublicKeyPath() string {
	pubkeyPath, err := path.ExpandLocalHomeDirPath(defaultPublicKeyPath)
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
	privkeyPath, err := path.ExpandLocalHomeDirPath(defaultPrivateKeyPath)
	if err != nil {
		return ""
	}

	return privkeyPath
}

// DecodePublicPrivateKey decodes public or private key
func DecodePublicPrivateKey(keyPath string) (interface{}, error) {
	pemBytes, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read public/private key file %q", keyPath)
	}

	// is pem?
	block, _ := pem.Decode(pemBytes)
	if block != nil {
		if strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return nil, errors.New("PEM blocks are encrypted")
		}

		switch block.Type {
		case "RSA PRIVATE KEY", "PRIVATE KEY", "EC PRIVATE KEY", "DSA PRIVATE KEY", "OPENSSH PRIVATE KEY":
			privateKey, err := ssh.ParseRawPrivateKey(pemBytes)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse private key file %q", keyPath)
			}

			return privateKey, nil
		case "RSA PUBLIC KEY", "PUBLIC KEY", "EC PUBLIC KEY", "DSA PUBLIC KEY", "OPENSSH PUBLIC KEY":
			publicKey, err := ssh.ParsePublicKey(pemBytes)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse public key file %q", keyPath)
			}

			return publicKey, nil
		default:
			return nil, errors.Wrapf(err, "failed to parse public/private key file %q", keyPath)
		}
	}

	// authorized key
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(pemBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse public key file %q", keyPath)
	}

	parsedCryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
	if !ok {
		return nil, errors.New("failed to get crypto public key")
	}

	pubCrypto := parsedCryptoKey.CryptoPublicKey()
	pubKey, ok := pubCrypto.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("failed to get RSA public key")
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

	return nil, errors.New("failed to get private key")
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

	return nil, errors.New("failed to get public key")
}
