package encryption

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"testing"

	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons/path"

	"github.com/stretchr/testify/assert"
)

type readerWithDataAndEOF struct {
	data []byte
	read bool
}

func (reader *readerWithDataAndEOF) Read(buffer []byte) (int, error) {
	if reader.read {
		return 0, io.EOF
	}

	reader.read = true
	return copy(buffer, reader.data), io.EOF
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) {
	return 0, nil
}

func TestEncrypt(t *testing.T) {
	t.Run("test EncryptFilenamePGP", testEncryptFilenamePGP)
	t.Run("test EncryptFilenameWinSCP", testEncryptFilenameWinSCP)
	t.Run("test DecryptFilenameWinSCP", testDecryptFilenameWinSCP)
	t.Run("test EncryptFilenameSSH", testEncryptFilenameSSH)
	t.Run("test EncryptFilePGP", testEncryptFilePGP)
	t.Run("test EncryptFileWinSCP", testEncryptFileWinSCP)
	t.Run("test EncryptFileSSH", testEncryptFileSSH)
}

func TestAESCTRReaderWriter(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	salt := bytes.Repeat([]byte{0x22}, AesSaltLen)
	data := makeFixedContentTestDataBuf(aesCTRBufferSize + 7)

	var encrypted bytes.Buffer
	err := EncryptAESCTRReaderWriter(bytes.NewReader(data), &encrypted, salt, key)
	assert.NoError(t, err)
	assert.Len(t, encrypted.Bytes(), len(data))

	var decrypted bytes.Buffer
	err = DecryptAESCTRReaderWriter(&encrypted, &decrypted, salt, key)
	assert.NoError(t, err)
	assert.Equal(t, data, decrypted.Bytes())
}

func TestAESCTRReaderWriterPreservesDataReturnedWithEOF(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	salt := bytes.Repeat([]byte{0x22}, AesSaltLen)
	data := []byte("data returned with EOF must be encrypted")

	var encrypted bytes.Buffer
	err := EncryptAESCTRReaderWriter(&readerWithDataAndEOF{data: data}, &encrypted, salt, key)
	assert.NoError(t, err)

	var decrypted bytes.Buffer
	err = DecryptAESCTRReaderWriter(&encrypted, &decrypted, salt, key)
	assert.NoError(t, err)
	assert.Equal(t, data, decrypted.Bytes())
}

func TestAESCTRReaderWriterReturnsShortWrite(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, 32)
	salt := bytes.Repeat([]byte{0x22}, AesSaltLen)

	err := EncryptAESCTRReaderWriter(bytes.NewReader([]byte("data")), shortWriter{}, salt, key)
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

func makeFixedContentTestDataBuf(size int64) []byte {
	testval := "abcdefghijklmnopqrstuvwxyz"

	// fill
	dataBuf := make([]byte, size)
	writeLen := 0
	for writeLen < len(dataBuf) {
		copy(dataBuf[writeLen:], testval)
		writeLen += len(testval)
	}
	return dataBuf
}

func createLocalTestFile(name string, size int64) (string, error) {
	// fill
	dataBuf := makeFixedContentTestDataBuf(1024)

	f, err := os.CreateTemp("", name)
	if err != nil {
		return "", err
	}

	tempPath := f.Name()

	defer f.Close()

	totalWriteLen := int64(0)
	for totalWriteLen < size {
		writeLen, err := f.Write(dataBuf)
		if err != nil {
			os.Remove(tempPath)
			return "", err
		}

		totalWriteLen += int64(writeLen)
	}

	return tempPath, nil
}

func testEncryptFilenamePGP(t *testing.T) {
	filename := "test_large_file.bin"

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encryptManager := NewEncryptionManager(EncryptionModePGP)
	encryptManager.SetKey(passwordBytes)

	encFilename, err := encryptManager.EncryptFilename(filename)
	assert.NoError(t, err)

	decFilename, err := encryptManager.DecryptFilename(encFilename)
	assert.NoError(t, err)

	// compare
	assert.Equal(t, filename, decFilename)
}

func testDecryptFilenameWinSCP(t *testing.T) {
	filename := "fVten7j3mxzA0LVfDcLSkycYrFHQqEU.aesctr.enc"

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encryptManager := NewEncryptionManager(EncryptionModeWinSCP)
	encryptManager.SetKey(passwordBytes)

	decFilename, err := encryptManager.DecryptFilename(filename)
	assert.NoError(t, err)

	assert.Equal(t, "LICENSE", decFilename)
}

func testEncryptFilenameWinSCP(t *testing.T) {
	filename := "LICENSE"

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encryptManager := NewEncryptionManager(EncryptionModeWinSCP)
	encryptManager.SetKey(passwordBytes)

	encFilename, err := encryptManager.EncryptFilename(filename)
	assert.NoError(t, err)

	decFilename, err := encryptManager.DecryptFilename(encFilename)
	assert.NoError(t, err)

	// compare
	assert.Equal(t, filename, decFilename)
}

func testEncryptFilenameSSH(t *testing.T) {
	filename := "LICENSE"

	keypath, err := path.ExpandLocalHomeDirPath(defaultPrivateKeyPath)
	assert.NoError(t, err)

	encryptManager := NewEncryptionManager(EncryptionModeSSH)
	encryptManager.SetPublicPrivateKey(keypath)

	encFilename, err := encryptManager.EncryptFilename(filename)
	assert.NoError(t, err)

	decFilename, err := encryptManager.DecryptFilename(encFilename)
	assert.NoError(t, err)

	// compare
	assert.Equal(t, filename, decFilename)
}

func testEncryptFilePGP(t *testing.T) {
	fileSize := 10 * 1024 * 1024 // 10MB

	filename := "test_large_file.bin"
	filepath, err := createLocalTestFile(filename, int64(fileSize))
	assert.NoError(t, err)

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encFilePath := filepath + ".enc"
	decFilePath := filepath + ".dec"

	encryptManager := NewEncryptionManager(EncryptionModePGP)
	encryptManager.SetKey(passwordBytes)

	err = encryptManager.EncryptFile(filepath, encFilePath)
	assert.NoError(t, err)

	err = encryptManager.DecryptFile(encFilePath, decFilePath)
	assert.NoError(t, err)

	// compare
	sourceHash, err := irodsclient_util.HashLocalFile(filepath, "SHA-256", nil)
	assert.NoError(t, err)

	decHash, err := irodsclient_util.HashLocalFile(decFilePath, "SHA-256", nil)
	assert.NoError(t, err)

	assert.Equal(t, sourceHash, decHash)

	err = os.Remove(filepath)
	assert.NoError(t, err)

	err = os.Remove(encFilePath)
	assert.NoError(t, err)

	err = os.Remove(decFilePath)
	assert.NoError(t, err)
}

func testEncryptFileWinSCP(t *testing.T) {
	fileSize := 10 * 1024 * 1024 // 10MB

	filename := "test_large_file.bin"
	filepath, err := createLocalTestFile(filename, int64(fileSize))
	assert.NoError(t, err)

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encFilePath := filepath + ".enc"
	decFilePath := filepath + ".dec"

	encryptManager := NewEncryptionManager(EncryptionModeWinSCP)
	encryptManager.SetKey(passwordBytes)

	err = encryptManager.EncryptFile(filepath, encFilePath)
	assert.NoError(t, err)

	err = encryptManager.DecryptFile(encFilePath, decFilePath)
	assert.NoError(t, err)

	// compare
	sourceHash, err := irodsclient_util.HashLocalFile(filepath, "SHA-256", nil)
	assert.NoError(t, err)

	decHash, err := irodsclient_util.HashLocalFile(decFilePath, "SHA-256", nil)
	assert.NoError(t, err)

	assert.Equal(t, sourceHash, decHash)

	err = os.Remove(filepath)
	assert.NoError(t, err)

	err = os.Remove(encFilePath)
	assert.NoError(t, err)

	err = os.Remove(decFilePath)
	assert.NoError(t, err)
}

func testEncryptFileSSH(t *testing.T) {
	fileSize := 10 * 1024 * 1024 // 10MB

	filename := "test_large_file.bin"
	filepath, err := createLocalTestFile(filename, int64(fileSize))
	assert.NoError(t, err)

	encFilePath := filepath + ".enc"
	decFilePath := filepath + ".dec"

	keypath, err := path.ExpandLocalHomeDirPath(defaultPrivateKeyPath)
	assert.NoError(t, err)

	encryptManager := NewEncryptionManager(EncryptionModeSSH)
	encryptManager.SetPublicPrivateKey(keypath)

	err = encryptManager.EncryptFile(filepath, encFilePath)
	assert.NoError(t, err)

	err = encryptManager.DecryptFile(encFilePath, decFilePath)
	assert.NoError(t, err)

	// compare
	sourceHash, err := irodsclient_util.HashLocalFile(filepath, "SHA-256", nil)
	assert.NoError(t, err)

	decHash, err := irodsclient_util.HashLocalFile(decFilePath, "SHA-256", nil)
	assert.NoError(t, err)

	assert.Equal(t, sourceHash, decHash)

	err = os.Remove(filepath)
	assert.NoError(t, err)

	err = os.Remove(encFilePath)
	assert.NoError(t, err)

	err = os.Remove(decFilePath)
	assert.NoError(t, err)
}
