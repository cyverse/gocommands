package encryption

import (
	"fmt"
	"io"
	"os"
	"strings"

	_ "crypto/sha256"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
	_ "golang.org/x/crypto/ripemd160"
	"golang.org/x/xerrors"
)

const (
	PgpEncryptedFileExtension string = ".pgp.enc"

	PgpSalt string = "4e2f34041d564ed8"
)

func EncryptFilenamePGP(filename string) string {
	return fmt.Sprintf("%s%s", filename, PgpEncryptedFileExtension)
}

func DecryptFilenamePGP(filename string) string {
	// trim file ext
	return strings.TrimSuffix(filename, PgpEncryptedFileExtension)
}

func EncryptFilePGP(source string, target string, key []byte) error {
	sourceFileHandle, err := os.Open(source)
	if err != nil {
		return xerrors.Errorf("failed to open file %q: %w", source, err)
	}

	defer sourceFileHandle.Close()

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return xerrors.Errorf("failed to create file %q: %w", target, err)
	}

	defer targetFileHandle.Close()

	encryptionConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}

	writeHandle, err := openpgp.SymmetricallyEncrypt(targetFileHandle, key, nil, encryptionConfig)
	if err != nil {
		return xerrors.Errorf("failed to create a encrypt writer for %q: %w", target, err)
	}

	defer writeHandle.Close()

	_, err = io.Copy(writeHandle, sourceFileHandle)
	if err != nil {
		return xerrors.Errorf("failed to encrypt data: %w", err)
	}

	return nil
}

func DecryptFilePGP(source string, target string, key []byte) error {
	sourceFileHandle, err := os.Open(source)
	if err != nil {
		return xerrors.Errorf("failed to open file %q: %w", source, err)
	}

	defer sourceFileHandle.Close()

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return xerrors.Errorf("failed to create file %q: %w", target, err)
	}

	defer targetFileHandle.Close()

	encryptionConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}

	failed := false
	prompt := func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if failed {
			return nil, xerrors.New("decryption failed")
		}
		failed = true
		return key, nil
	}

	messageDetail, err := openpgp.ReadMessage(sourceFileHandle, nil, prompt, encryptionConfig)
	if err != nil {
		return xerrors.Errorf("failed to decrypt for %q: %w", source, err)
	}

	_, err = io.Copy(targetFileHandle, messageDetail.UnverifiedBody)
	if err != nil {
		return xerrors.Errorf("failed to decrypt data: %w", err)
	}

	return nil
}
