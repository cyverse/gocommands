package encryption

import (
	"fmt"
	"io"
	"os"
	"strings"

	_ "crypto/sha256"

	"github.com/cockroachdb/errors"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
	_ "golang.org/x/crypto/ripemd160"
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
		return errors.Wrapf(err, "failed to open file %q", source)
	}

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		sourceFileHandle.Close()
		return errors.Wrapf(err, "failed to create file %q", target)
	}

	encryptionConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}

	writeHandle, err := openpgp.SymmetricallyEncrypt(targetFileHandle, key, nil, encryptionConfig)
	if err != nil {
		targetFileHandle.Close()
		sourceFileHandle.Close()
		return errors.Wrapf(err, "failed to create a encrypt writer for %q", target)
	}

	_, err = io.Copy(writeHandle, sourceFileHandle)
	if err != nil {
		writeHandle.Close()
		targetFileHandle.Close()
		sourceFileHandle.Close()
		return errors.Wrapf(err, "failed to encrypt data")
	}
	if err := writeHandle.Close(); err != nil {
		targetFileHandle.Close()
		sourceFileHandle.Close()
		return errors.Wrapf(err, "failed to finalize encrypted file %q", target)
	}
	if err := targetFileHandle.Close(); err != nil {
		sourceFileHandle.Close()
		return errors.Wrapf(err, "failed to close file %q", target)
	}
	if err := sourceFileHandle.Close(); err != nil {
		return errors.Wrapf(err, "failed to close file %q", source)
	}

	return nil
}

func DecryptFilePGP(source string, target string, key []byte) error {
	sourceFileHandle, err := os.Open(source)
	if err != nil {
		return errors.Wrapf(err, "failed to open file %q", source)
	}

	defer sourceFileHandle.Close()

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %q", target)
	}

	defer targetFileHandle.Close()

	encryptionConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}

	failed := false
	prompt := func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if failed {
			return nil, errors.New("decryption failed")
		}
		failed = true
		return key, nil
	}

	messageDetail, err := openpgp.ReadMessage(sourceFileHandle, nil, prompt, encryptionConfig)
	if err != nil {
		return errors.Wrapf(err, "failed to decrypt for %q", source)
	}

	_, err = io.Copy(targetFileHandle, messageDetail.UnverifiedBody)
	if err != nil {
		return errors.Wrapf(err, "failed to decrypt data for %q", source)
	}

	return nil
}
