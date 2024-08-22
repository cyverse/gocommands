package commons

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/xerrors"
)

// For WinSCP encryption
// https://winscp.net/eng/docs/file_encryption

const (
	WinSCPEncryptedFileExtension string = ".aesctr.enc"

	WinSCPAesCtrHeader string = "aesctr.........."
)

func EncryptFilenameWinSCP(filename string, key []byte) (string, error) {
	// generate salt
	salt := make([]byte, AesSaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", xerrors.Errorf("failed to generate salt: %w", err)
	}

	// convert to utf8
	utf8Filename := strings.ToValidUTF8(filename, "_")

	// encrypt with aes 256 ctr
	encryptedFilename, err := EncryptAESCTR([]byte(utf8Filename), salt, key)
	if err != nil {
		return "", xerrors.Errorf("failed to encrypt filename: %w", err)
	}

	// add salt in front
	concatenatedFilename := make([]byte, len(salt)+len(encryptedFilename))
	copy(concatenatedFilename, salt)
	copy(concatenatedFilename[len(salt):], encryptedFilename)

	// base64 encode
	b64EncodedFilename := base64.RawStdEncoding.EncodeToString(concatenatedFilename)
	// replace / to _
	b64EncodedFilename = strings.ReplaceAll(b64EncodedFilename, "/", "_")

	newFilename := fmt.Sprintf("%s%s", b64EncodedFilename, WinSCPEncryptedFileExtension)

	return newFilename, nil
}

func DecryptFilenameWinSCP(filename string, key []byte) (string, error) {
	// trim file ext
	filename = strings.TrimSuffix(filename, WinSCPEncryptedFileExtension)

	// replace _ to /
	filename = strings.ReplaceAll(filename, "_", "/")

	// base64 decode
	concatenatedFilename, err := base64.RawStdEncoding.DecodeString(string(filename))
	if err != nil {
		return "", xerrors.Errorf("failed to base64 decode filename: %w", err)
	}

	if len(concatenatedFilename) < AesSaltLen {
		return "", xerrors.Errorf("failed to extract salt from filename")
	}

	salt := concatenatedFilename[:AesSaltLen]
	encryptedFilename := concatenatedFilename[AesSaltLen:]

	// decrypt with aes 256 ctr
	decryptedFilename, err := DecryptAESCTR(encryptedFilename, salt, key)
	if err != nil {
		return "", xerrors.Errorf("failed to decrypt filename: %w", err)
	}

	if !IsCorrectFilename(decryptedFilename) {
		return "", xerrors.Errorf("failed to decrypt filename with wrong key")
	}

	return string(decryptedFilename), nil
}

func EncryptFileWinSCP(source string, target string, key []byte) error {
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

	stat, err := sourceFileHandle.Stat()
	if err != nil {
		return xerrors.Errorf("failed to stat file %q: %w", source, err)
	}

	if stat.Size() == 0 {
		// empty file
		return nil
	}

	// write header
	_, err = targetFileHandle.Write([]byte(WinSCPAesCtrHeader))
	if err != nil {
		return xerrors.Errorf("failed to write header: %w", err)
	}

	// generate salt
	// we should use static salt to keep the same file name
	salt := make([]byte, AesSaltLen)
	_, err = rand.Read(salt)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return xerrors.Errorf("failed to read random data: %w", err)
	}

	// write salt
	_, err = targetFileHandle.Write(salt)
	if err != nil {
		return xerrors.Errorf("failed to write salt: %w", err)
	}

	err = EncryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt, key)
	if err != nil {
		return xerrors.Errorf("failed to encrypt file content: %w", err)
	}

	return nil
}

func DecryptFileWinSCP(source string, target string, key []byte) error {
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

	header := make([]byte, 16)

	readLen, err := sourceFileHandle.Read(header)
	if err == io.EOF && readLen == 0 {
		return nil
	}

	if err != nil {
		return xerrors.Errorf("failed to read AES CTR header: %w", err)
	}

	if !bytes.Equal(header, []byte(WinSCPAesCtrHeader)) {
		return xerrors.Errorf("failed to read AES CTR header")
	}

	salt := make([]byte, AesSaltLen)
	readLen, err = sourceFileHandle.Read(salt)
	if err != nil {
		return xerrors.Errorf("failed to read salt: %w", err)
	}

	if readLen != AesSaltLen {
		return xerrors.Errorf("failed to read salt, read len %d: %w", readLen, err)
	}

	err = DecryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt, key)
	if err != nil {
		return xerrors.Errorf("failed to decrypt file content: %w", err)
	}

	return nil
}
