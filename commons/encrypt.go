package commons

import (
	"crypto/rsa"
	"strings"

	"golang.org/x/xerrors"
)

// EncryptionMode determines encryption mode
type EncryptionMode string

const (
	// EncryptionModeWinSCP is for WinSCP
	EncryptionModeWinSCP EncryptionMode = "WINSCP"
	// EncryptionModePGP is for PGP key encryption
	EncryptionModePGP EncryptionMode = "PGP"
	// EncryptionModeSSH is for SSH key encryption
	EncryptionModeSSH EncryptionMode = "SSH"
	// EncryptionModeUnknown is for unknown mode
	EncryptionModeUnknown EncryptionMode = ""
)

// GetEncryptionMode returns encryption mode
func GetEncryptionMode(mode string) EncryptionMode {
	switch strings.ToUpper(mode) {
	case string(EncryptionModeWinSCP), "AES":
		return EncryptionModeWinSCP
	case string(EncryptionModePGP), "GPG", "OPENPGP":
		return EncryptionModePGP
	case string(EncryptionModeSSH):
		return EncryptionModeSSH
	default:
		return EncryptionModeUnknown
	}
}

// DetectEncryptionMode detects encryption mode and filename encryption
func DetectEncryptionMode(p string) EncryptionMode {
	if strings.HasSuffix(p, PgpEncryptedFileExtension) {
		// pgp
		return EncryptionModePGP
	} else if strings.HasSuffix(p, WinSCPEncryptedFileExtension) {
		// winscp
		return EncryptionModeWinSCP
	} else if strings.HasSuffix(p, SshEncryptedFileExtension) {
		// ssh
		return EncryptionModeSSH
	} else {
		return EncryptionModeUnknown
	}
}

func IsCorrectFilename(filename []byte) bool {
	for _, c := range filename {
		if c < 32 || c >= 126 {
			return false
		}
	}

	return true
}

type EncryptionManager struct {
	mode           EncryptionMode
	key            []byte
	privateKeyPath string
	publicKeyPath  string
}

// NewEncryptionManager creates a new EncryptionManager
func NewEncryptionManager(mode EncryptionMode) *EncryptionManager {
	manager := &EncryptionManager{
		mode: mode,
	}

	return manager
}

// SetKey sets key
func (manager *EncryptionManager) SetKey(key []byte) {
	manager.key = key
}

// SetPrivateKey sets private key
func (manager *EncryptionManager) SetPrivateKey(privateKeyPath string) {
	manager.privateKeyPath = privateKeyPath
}

// SetPublicKey sets public key
func (manager *EncryptionManager) SetPublicKey(publicKeyPath string) {
	manager.publicKeyPath = publicKeyPath
}

func (manager *EncryptionManager) getPublicKey() (*rsa.PublicKey, error) {
	if len(manager.publicKeyPath) > 0 {
		return DecodePublicKey(manager.publicKeyPath)
	}

	if len(manager.privateKeyPath) > 0 {
		priv, err := DecodePrivateKey(manager.privateKeyPath)
		if err != nil {
			return nil, err
		}

		return &priv.PublicKey, nil
	}

	return nil, xerrors.Errorf("failed to load public key, public or private key path is not given")
}

func (manager *EncryptionManager) getPrivateKey() (*rsa.PrivateKey, error) {
	if len(manager.privateKeyPath) > 0 {
		priv, err := DecodePrivateKey(manager.privateKeyPath)
		if err != nil {
			return nil, err
		}

		return priv, nil
	}

	return nil, xerrors.Errorf("failed to load private key, private key path is not given")
}

// EncryptFilename encrypts filename
func (manager *EncryptionManager) EncryptFilename(filename string) (string, error) {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return EncryptFilenameWinSCP(filename, manager.key)
	case EncryptionModePGP:
		return EncryptFilenamePGP(filename), nil
	case EncryptionModeSSH:
		// load publickey
		publicKey, err := manager.getPublicKey()
		if err != nil {
			return "", err
		}

		return EncryptFilenameSSH(filename, publicKey)
	default:
		return "", xerrors.Errorf("unknown encryption mode")
	}
}

// DecryptFilename decrypts filename
func (manager *EncryptionManager) DecryptFilename(filename string) (string, error) {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return DecryptFilenameWinSCP(filename, manager.key)
	case EncryptionModePGP:
		return DecryptFilenamePGP(filename), nil
	case EncryptionModeSSH:
		// load publickey
		privateKey, err := manager.getPrivateKey()
		if err != nil {
			return "", err
		}

		return DecryptFilenameSSH(filename, privateKey)
	default:
		return "", xerrors.Errorf("unknown encryption mode")
	}
}

// EncryptFile encrypts local source file and returns encrypted file path
func (manager *EncryptionManager) EncryptFile(source string, target string) error {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return EncryptFileWinSCP(source, target, manager.key)
	case EncryptionModePGP:
		return EncryptFilePGP(source, target, manager.key)
	case EncryptionModeSSH:
		// load publickey
		publicKey, err := manager.getPublicKey()
		if err != nil {
			return err
		}

		return EncryptFileSSH(source, target, publicKey)
	default:
		return xerrors.Errorf("unknown encryption mode")
	}
}

// DecryptFile decrypts local source file and returns decrypted file path
func (manager *EncryptionManager) DecryptFile(source string, target string) error {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return DecryptFileWinSCP(source, target, manager.key)
	case EncryptionModePGP:
		return DecryptFilePGP(source, target, manager.key)
	case EncryptionModeSSH:
		// load publickey
		privateKey, err := manager.getPrivateKey()
		if err != nil {
			return err
		}

		return DecryptFileSSH(source, target, privateKey)
	default:
		return xerrors.Errorf("unknown encryption mode")
	}
}
