package commons

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/jxskiss/base62"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/xerrors"
)

// AES key must be 16bytes len
func padAesKey(key string) string {
	paddedKey := fmt.Sprintf("%s%s", key, aesPadding)
	return paddedKey[:16]
}

func padPkcs7(data []byte, blocksize int) []byte {
	n := blocksize - (len(data) % blocksize)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))
	return pb
}

func AesDecrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCBCDecrypter(block, []byte(aesIV))
	contentLength := binary.LittleEndian.Uint32(data[:4])

	dest := make([]byte, len(data[4:]))
	decrypter.CryptBlocks(dest, data[4:])

	return dest[:contentLength], nil
}

func AesEncrypt(key string, data []byte) ([]byte, error) {
	key = padAesKey(key)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	encrypter := cipher.NewCBCEncrypter(block, []byte(aesIV))

	contentLength := uint32(len(data))
	padData := padPkcs7(data, block.BlockSize())

	dest := make([]byte, len(padData)+4)

	// add size header
	binary.LittleEndian.PutUint32(dest, contentLength)
	encrypter.CryptBlocks(dest[4:], padData)

	return dest, nil
}

func EncryptFilenameWithPassword(filename string, password string) (string, error) {
	encryptedFilename, err := AesEncrypt(password, []byte(filename))
	if err != nil {
		xerrors.Errorf("failed to encrypt filename: %w", err)
	}

	return base62.EncodeToString(encryptedFilename), nil
}

func DecryptFilenameWithPassword(filename string, password string) (string, error) {
	encryptedFilename, err := base62.DecodeString(string(filename))
	if err != nil {
		xerrors.Errorf("failed to base62 decode filename: %w", err)
	}

	decryptedFilename, err := AesDecrypt(password, encryptedFilename)
	if err != nil {
		xerrors.Errorf("failed to decrypt filename: %w", err)
	}

	return string(decryptedFilename), nil
}

func PgpEncryptFileWithPassword(source string, target string, password string) error {
	sourceFileHandle, err := os.Open(source)
	if err != nil {
		return xerrors.Errorf("failed to open file %s: %w", source, err)
	}

	defer sourceFileHandle.Close()

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return xerrors.Errorf("failed to create file %s: %w", target, err)
	}

	defer targetFileHandle.Close()

	encryptionConfig := &packet.Config{
		DefaultCipher: packet.CipherAES256,
	}

	writeHandle, err := openpgp.SymmetricallyEncrypt(targetFileHandle, []byte(password), nil, encryptionConfig)
	if err != nil {
		return xerrors.Errorf("failed to create a encrypt writer for %s: %w", target, err)
	}

	defer writeHandle.Close()

	_, err = io.Copy(writeHandle, sourceFileHandle)
	if err != nil {
		return xerrors.Errorf("failed to encrypt data: %w", err)
	}

	return nil
}

func PgpDecryptFileWithPassword(source string, target string, password string) error {
	sourceFileHandle, err := os.Open(source)
	if err != nil {
		return xerrors.Errorf("failed to open file %s: %w", source, err)
	}

	defer sourceFileHandle.Close()

	targetFileHandle, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return xerrors.Errorf("failed to create file %s: %w", target, err)
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
		return []byte(password), nil
	}

	messageDetail, err := openpgp.ReadMessage(sourceFileHandle, nil, prompt, encryptionConfig)
	if err != nil {
		return xerrors.Errorf("failed to decrypt for %s: %w", source, err)
	}

	_, err = io.Copy(targetFileHandle, messageDetail.UnverifiedBody)
	if err != nil {
		return xerrors.Errorf("failed to decrypt data: %w", err)
	}

	return nil
}
