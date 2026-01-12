package encryption

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	_ "crypto/sha256"

	"github.com/cockroachdb/errors"
	_ "golang.org/x/crypto/ripemd160"
)

const (
	SshEncryptedFileExtension string = ".rsaaesctr.enc"
	SshRsaAesCtrHeader        string = "rsaaesctr......."
)

func EncryptFilenameSSH(filename string, publickey *rsa.PublicKey) (string, error) {
	// generate salt
	salt := make([]byte, AesSaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", errors.Wrapf(err, "failed to generate salt")
	}

	// convert to utf8
	utf8Filename := strings.ToValidUTF8(filename, "_")

	// we extract N from public key and use it as shared key for AES CTR
	// this is because max length of filename is limited, while general RSA generates very long encrypted bytes

	// encrypt with aes 256 ctr
	encryptedFilename, err := EncryptAESCTR([]byte(utf8Filename), salt, publickey.N.Bytes()[:32])
	if err != nil {
		return "", errors.Wrapf(err, "failed to encrypt filename")
	}

	// add salt in front
	concatenatedFilename := make([]byte, len(salt)+len(encryptedFilename))
	copy(concatenatedFilename, salt)
	copy(concatenatedFilename[len(salt):], encryptedFilename)

	// base64 encode
	b64EncodedFilename := base64.RawStdEncoding.EncodeToString(concatenatedFilename)
	// replace / to _
	b64EncodedFilename = strings.ReplaceAll(b64EncodedFilename, "/", "_")

	newFilename := fmt.Sprintf("%s%s", b64EncodedFilename, SshEncryptedFileExtension)

	return newFilename, nil
}

func DecryptFilenameSSH(filename string, privatekey *rsa.PrivateKey) (string, error) {
	// trim file ext
	filename = strings.TrimSuffix(filename, SshEncryptedFileExtension)

	// replace _ to /
	filename = strings.ReplaceAll(filename, "_", "/")

	// base64 decode
	concatenatedFilename, err := base64.RawStdEncoding.DecodeString(string(filename))
	if err != nil {
		return "", errors.Wrapf(err, "failed to base64 decode filename")
	}

	if len(concatenatedFilename) < AesSaltLen {
		return "", errors.New("failed to extract salt from filename")
	}

	salt := concatenatedFilename[:AesSaltLen]
	encryptedFilename := concatenatedFilename[AesSaltLen:]

	// decrypt with aes 256 ctr
	decryptedFilename, err := DecryptAESCTR(encryptedFilename, salt, privatekey.PublicKey.N.Bytes()[:32])
	if err != nil {
		return "", errors.Wrapf(err, "failed to decrypt filename")
	}

	if !IsCorrectFilename(decryptedFilename) {
		return "", errors.New("failed to decrypt filename with wrong key")
	}

	return string(decryptedFilename), nil
}

func EncryptFileSSH(source string, target string, publickey *rsa.PublicKey) error {
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

	stat, err := sourceFileHandle.Stat()
	if err != nil {
		return errors.Wrapf(err, "failed to stat file %q", source)
	}

	if stat.Size() == 0 {
		// empty file
		return nil
	}

	// write header
	_, err = targetFileHandle.Write([]byte(SshRsaAesCtrHeader))
	if err != nil {
		return errors.Wrapf(err, "failed to write header")
	}

	// generate salt
	salt := make([]byte, AesSaltLen)
	_, err = rand.Read(salt)
	if err != nil {
		return errors.Wrapf(err, "failed to read random data")
	}

	// generate shared key
	sharedKey := make([]byte, 32)
	_, err = rand.Read(sharedKey)
	if err != nil {
		return errors.Wrapf(err, "failed to generate random shared key")
	}

	headerBuffer := make([]byte, AesSaltLen+32)
	copy(headerBuffer[:AesSaltLen], salt)
	copy(headerBuffer[AesSaltLen:], sharedKey)

	// RSA encrypt
	oaepLabel := []byte("")
	encryptedHeader, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, publickey, headerBuffer, oaepLabel)
	if err != nil {
		return errors.Wrapf(err, "failed to encrypt header")
	}

	// write header len
	lenBuffer := make([]byte, 32)
	binary.LittleEndian.PutUint32(lenBuffer, uint32(len(encryptedHeader)))
	_, err = targetFileHandle.Write(lenBuffer)
	if err != nil {
		return errors.Wrapf(err, "failed to write encrypted header length")
	}

	// write salt and shared key
	_, err = targetFileHandle.Write(encryptedHeader)
	if err != nil {
		return errors.Wrapf(err, "failed to write encrypted header")
	}

	err = EncryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt, sharedKey)
	if err != nil {
		return errors.Wrapf(err, "failed to encrypt file content")
	}

	return nil
}

func DecryptFileSSH(source string, target string, privatekey *rsa.PrivateKey) error {
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

	header := make([]byte, 16)
	readLen, err := sourceFileHandle.Read(header)
	if err == io.EOF && readLen == 0 {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "failed to read RSA AES CTR header")
	}

	if !bytes.Equal(header, []byte(SshRsaAesCtrHeader)) {
		return errors.New("failed to read RSA AES CTR header")
	}

	lenBuffer := make([]byte, 32)
	readLen, err = sourceFileHandle.Read(lenBuffer)
	if err != nil {
		return errors.Wrapf(err, "failed to read encrypted header length")
	}

	if readLen != 32 {
		return errors.New("failed to read encrypted header length")
	}

	encryptedHeaderLength := binary.LittleEndian.Uint32(lenBuffer)
	encryptedHeaderBuffer := make([]byte, encryptedHeaderLength)
	readLen, err = sourceFileHandle.Read(encryptedHeaderBuffer)
	if err != nil {
		return errors.Wrapf(err, "failed to read encrypted header")
	}

	if readLen != int(encryptedHeaderLength) {
		return errors.New("failed to read encrypted header")
	}

	// RSA decrypt
	oaepLabel := []byte("")
	decryptedHeader, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privatekey, encryptedHeaderBuffer, oaepLabel)
	if err != nil {
		return errors.Wrapf(err, "failed to decrypt header")
	}

	if len(decryptedHeader) != AesSaltLen+32 {
		return errors.New("failed to decrypt header")
	}

	salt := decryptedHeader[:AesSaltLen]
	sharedKey := decryptedHeader[AesSaltLen:]

	err = DecryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt, sharedKey)
	if err != nil {
		return errors.Wrapf(err, "failed to decrypt file content")
	}

	return nil
}
