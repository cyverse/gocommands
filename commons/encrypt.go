package commons

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jxskiss/base62"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/packet"
	"golang.org/x/xerrors"
)

// For WinSCP encryption
// https://winscp.net/eng/docs/file_encryption
// not available on golang for now

const (
	PgpEncryptedFileExtension    string = ".pgp.enc"
	WinSCPEncryptedFileExtension string = ".aesctr.enc"

	aesSaltLen         int    = 16
	pgpSalt            string = "4e2f34041d564ed8"
	winScpAesCtrHeader string = "aesctr.........."
)

// EncryptionMode determines encryption mode
type EncryptionMode string

const (
	// EncryptionModeWinSCP is for WinSCP
	EncryptionModeWinSCP EncryptionMode = "WINSCP"
	// EncryptionModePGP is for PGP
	EncryptionModePGP EncryptionMode = "PGP"
	// EncryptionModeUnknown is for unknown mode
	EncryptionModeUnknown EncryptionMode = ""
)

// GetEncryptionMode returns encryption mode
func GetEncryptionMode(mode string) EncryptionMode {
	switch strings.ToUpper(mode) {
	case string(EncryptionModeWinSCP):
		return EncryptionModeWinSCP
	case string(EncryptionModePGP), "GPG", "OPENPGP":
		return EncryptionModePGP
	default:
		return EncryptionModeUnknown
	}
}

// DetectEncryptionMode detects encryption mode and filename encryption
func DetectEncryptionMode(p string) (EncryptionMode, bool) {
	if strings.HasSuffix(p, PgpEncryptedFileExtension) {
		// pgp
		return EncryptionModePGP, true
	} else if strings.HasSuffix(p, WinSCPEncryptedFileExtension) {
		// winscp
		return EncryptionModeWinSCP, true
	} else {
		return EncryptionModePGP, false
	}
}

type EncryptionManager struct {
	mode            EncryptionMode
	encryptFilename bool
	key             []byte
}

// NewEncryptionManager creates a new EncryptionManager
func NewEncryptionManager(mode EncryptionMode, encryptFilename bool, key []byte) *EncryptionManager {
	manager := &EncryptionManager{
		mode:            mode,
		encryptFilename: encryptFilename,
		key:             key,
	}

	return manager
}

func (manager *EncryptionManager) EncryptFilename(filename string) (string, error) {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return manager.encryptFilenameWinSCP(filename)
	case EncryptionModePGP:
		return manager.encryptFilenamePGP(filename)
	default:
		return manager.encryptFilenamePGP(filename)
	}
}

func (manager *EncryptionManager) DecryptFilename(filename string) (string, error) {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return manager.decryptFilenameWinSCP(filename)
	case EncryptionModePGP:
		return manager.decryptFilenamePGP(filename)
	default:
		return manager.decryptFilenamePGP(filename)
	}
}

// EncryptFile encrypts local source file and returns encrypted file path
func (manager *EncryptionManager) EncryptFile(source string, target string) error {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return manager.encryptFileWinSCP(source, target)
	case EncryptionModePGP:
		return manager.encryptFilePGP(source, target)
	default:
		return manager.encryptFilePGP(source, target)
	}
}

// DecryptFile decrypts local source file and returns decrypted file path
func (manager *EncryptionManager) DecryptFile(source string, target string) error {
	switch manager.mode {
	case EncryptionModeWinSCP:
		return manager.decryptFileWinSCP(source, target)
	case EncryptionModePGP:
		return manager.decryptFilePGP(source, target)
	default:
		return manager.decryptFilePGP(source, target)
	}
}

func (manager *EncryptionManager) encryptFilenameWinSCP(filename string) (string, error) {
	// generate salt
	salt := make([]byte, aesSaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", xerrors.Errorf("failed to generate salt: %w", err)
	}

	// convert to utf8
	utf8Filename := strings.ToValidUTF8(filename, "_")

	// encrypt with aes 256 ctr
	encryptedFilename, err := manager.encryptAESCTR([]byte(utf8Filename), salt)
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

func (manager *EncryptionManager) decryptFilenameWinSCP(filename string) (string, error) {
	// trim file ext
	filename = strings.TrimSuffix(filename, WinSCPEncryptedFileExtension)

	// replace _ to /
	filename = strings.ReplaceAll(filename, "_", "/")

	// base64 decode
	concatenatedFilename, err := base64.RawStdEncoding.DecodeString(string(filename))
	if err != nil {
		return "", xerrors.Errorf("failed to base64 decode filename: %w", err)
	}

	if len(concatenatedFilename) < aesSaltLen {
		return "", xerrors.Errorf("failed to extract salt from filename")
	}

	salt := concatenatedFilename[:aesSaltLen]
	encryptedFilename := concatenatedFilename[aesSaltLen:]

	// decrypt with aes 256 ctr
	decryptedFilename, err := manager.decryptAESCTR(encryptedFilename, salt)
	if err != nil {
		return "", xerrors.Errorf("failed to decrypt filename: %w", err)
	}

	return string(decryptedFilename), nil
}

func (manager *EncryptionManager) encryptFilenamePGP(filename string) (string, error) {
	if !manager.encryptFilename {
		return filename, nil
	}

	// generate salt
	// we should use static salt to keep the same file name
	salt := make([]byte, aesSaltLen)
	copy(salt, []byte(pgpSalt))

	encryptedFilename, err := manager.encryptAESCBC([]byte(filename), salt)
	if err != nil {
		return "", xerrors.Errorf("failed to encrypt filename: %w", err)
	}

	// add salt in front
	concatenatedFilename := make([]byte, len(salt)+len(encryptedFilename))
	copy(concatenatedFilename, salt)
	copy(concatenatedFilename[len(salt):], encryptedFilename)

	// base62 encode
	b62EncodedFilename := base62.EncodeToString(concatenatedFilename)

	newFilename := fmt.Sprintf("%s%s", b62EncodedFilename, PgpEncryptedFileExtension)

	return newFilename, nil
}

func (manager *EncryptionManager) decryptFilenamePGP(filename string) (string, error) {
	if !manager.encryptFilename {
		return filename, nil
	}

	// trim file ext
	filename = strings.TrimSuffix(filename, PgpEncryptedFileExtension)

	// base62 decode
	concatenatedFilename, err := base62.DecodeString(string(filename))
	if err != nil {
		return "", xerrors.Errorf("failed to base62 decode filename: %w", err)
	}

	if len(concatenatedFilename) < aesSaltLen {
		return "", xerrors.Errorf("failed to extract salt from filename")
	}

	salt := concatenatedFilename[:aesSaltLen]
	encryptedFilename := concatenatedFilename[aesSaltLen:]

	decryptedFilename, err := manager.decryptAESCBC(encryptedFilename, salt)
	if err != nil {
		return "", xerrors.Errorf("failed to decrypt filename: %w", err)
	}

	return string(decryptedFilename), nil
}

func (manager *EncryptionManager) encryptFileWinSCP(source string, target string) error {
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

	stat, err := sourceFileHandle.Stat()
	if err != nil {
		return xerrors.Errorf("failed to stat file %s: %w", source, err)
	}

	if stat.Size() == 0 {
		// empty file
		return nil
	}

	// write header
	_, err = targetFileHandle.Write([]byte(winScpAesCtrHeader))
	if err != nil {
		return xerrors.Errorf("failed to write header: %w", err)
	}

	// generate salt
	// we should use static salt to keep the same file name
	salt := make([]byte, aesSaltLen)
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

	err = manager.encryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt)
	if err != nil {
		return xerrors.Errorf("failed to encrypt file content: %w", err)
	}

	return nil
}

func (manager *EncryptionManager) decryptFileWinSCP(source string, target string) error {
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

	header := make([]byte, 16)

	readLen, err := sourceFileHandle.Read(header)
	if err == io.EOF && readLen == 0 {
		return nil
	}

	if err != nil {
		return xerrors.Errorf("failed to read AES CTR header: %w", err)
	}

	if !bytes.Equal(header, []byte(winScpAesCtrHeader)) {
		return xerrors.Errorf("failed to read AES CTR header")
	}

	salt := make([]byte, aesSaltLen)
	readLen, err = sourceFileHandle.Read(salt)
	if err != nil {
		return xerrors.Errorf("failed to read salt: %w", err)
	}

	if readLen != aesSaltLen {
		return xerrors.Errorf("failed to read salt, read len %d: %w", readLen, err)
	}

	err = manager.decryptAESCTRReaderWriter(sourceFileHandle, targetFileHandle, salt)
	if err != nil {
		return xerrors.Errorf("failed to decrypt file content: %w", err)
	}

	return nil
}

func (manager *EncryptionManager) encryptFilePGP(source string, target string) error {
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

	writeHandle, err := openpgp.SymmetricallyEncrypt(targetFileHandle, []byte(manager.key), nil, encryptionConfig)
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

func (manager *EncryptionManager) decryptFilePGP(source string, target string) error {
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
		return []byte(manager.key), nil
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

func (manager *EncryptionManager) padPkcs7(data []byte, blocksize int) []byte {
	if (len(data) % blocksize) == 0 {
		return data
	}

	n := blocksize - (len(data) % blocksize)
	pb := make([]byte, len(data)+n)
	copy(pb, data)
	copy(pb[len(data):], bytes.Repeat([]byte{byte(n)}, n))
	return pb
}

func (manager *EncryptionManager) encryptAESCBC(data []byte, salt []byte) ([]byte, error) {
	key := manager.padPkcs7(manager.key, 16)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	encrypter := cipher.NewCBCEncrypter(block, salt)

	contentLength := uint32(len(data))
	padData := manager.padPkcs7(data, block.BlockSize())

	dest := make([]byte, len(padData)+4)

	// add size header
	binary.LittleEndian.PutUint32(dest, contentLength)
	encrypter.CryptBlocks(dest[4:], padData)

	return dest, nil
}

func (manager *EncryptionManager) decryptAESCBC(data []byte, salt []byte) ([]byte, error) {
	key := manager.padPkcs7(manager.key, 16)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCBCDecrypter(block, salt)
	contentLength := binary.LittleEndian.Uint32(data[:4])

	dest := make([]byte, len(data[4:]))
	decrypter.CryptBlocks(dest, data[4:])

	return dest[:contentLength], nil
}

func (manager *EncryptionManager) encryptAESCTR(data []byte, salt []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	writerBuffer := &bytes.Buffer{}

	err := manager.encryptAESCTRReaderWriter(reader, writerBuffer, salt)
	if err != nil {
		return nil, err
	}

	return writerBuffer.Bytes(), nil
}

func (manager *EncryptionManager) decryptAESCTR(data []byte, salt []byte) ([]byte, error) {
	reader := bytes.NewReader(data)
	writerBuffer := &bytes.Buffer{}

	err := manager.decryptAESCTRReaderWriter(reader, writerBuffer, salt)
	if err != nil {
		return nil, err
	}

	return writerBuffer.Bytes(), nil
}

func (manager *EncryptionManager) encryptAESCTRReaderWriter(reader io.Reader, writer io.Writer, salt []byte) error {
	key := manager.padPkcs7(manager.key, 32)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCTR(block, salt)

	buf := make([]byte, block.BlockSize())
	destBuf := make([]byte, block.BlockSize())
	for {
		readLen, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		decrypter.XORKeyStream(destBuf, buf[:readLen])
		writeLen, err := writer.Write(destBuf[:readLen])
		if err != nil {
			return err
		}

		if writeLen != readLen {
			return xerrors.Errorf("failed to write")
		}
	}
}

func (manager *EncryptionManager) decryptAESCTRReaderWriter(reader io.Reader, writer io.Writer, salt []byte) error {
	key := manager.padPkcs7(manager.key, 32)
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return xerrors.Errorf("failed to create AES cipher: %w", err)
	}

	decrypter := cipher.NewCTR(block, salt)

	buf := make([]byte, block.BlockSize())
	destBuf := make([]byte, block.BlockSize())
	for {
		readLen, err := reader.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		decrypter.XORKeyStream(destBuf, buf[:readLen])
		writeLen, err := writer.Write(destBuf[:readLen])
		if err != nil {
			return err
		}

		if writeLen != readLen {
			return xerrors.Errorf("failed to write")
		}
	}
}
