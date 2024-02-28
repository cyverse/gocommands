package commons

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPGP(t *testing.T) {
	t.Run("test EncryptFilePGP", testEncryptFilePGP)
	t.Run("test EncryptFilenamePGP", testEncryptFilenamePGP)
	t.Run("test DecryptFilenameWinSCP", testDecryptFilenameWinSCP)
	t.Run("test EncryptFilenameWinSCP", testEncryptFilenameWinSCP)
}

func testEncryptFilePGP(t *testing.T) {
	testval := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // 62
	fileSize := 10 * 1024 * 1024                                                // 10MB

	filename := "test_large_file.bin"
	testFilePath, err := filepath.Abs(filename)
	assert.NoError(t, err)

	bufSize := 1024
	buf := make([]byte, bufSize)

	f, err := os.OpenFile(testFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
	assert.NoError(t, err)

	for i := 0; i < fileSize/bufSize; i++ {
		// fill buf
		for j := 0; j < bufSize; j++ {
			buf[j] = testval[j%len(testval)]
		}

		_, err = f.Write(buf)
		assert.NoError(t, err)
	}

	err = f.Close()
	assert.NoError(t, err)

	password := "test_password"
	encFilePath := testFilePath + ".enc"
	decFilePath := testFilePath + ".dec"

	encryptManager := NewEncryptionManager(EncryptionModePGP, false, []byte(password))

	err = encryptManager.EncryptFile(testFilePath, encFilePath)
	assert.NoError(t, err)

	err = encryptManager.DecryptFile(encFilePath, decFilePath)
	assert.NoError(t, err)

	// compare
	sourceHash, err := HashLocalFile(testFilePath, "SHA-256")
	assert.NoError(t, err)

	decHash, err := HashLocalFile(decFilePath, "SHA-256")
	assert.NoError(t, err)

	assert.Equal(t, sourceHash, decHash)

	err = os.Remove(testFilePath)
	assert.NoError(t, err)

	err = os.Remove(encFilePath)
	assert.NoError(t, err)

	err = os.Remove(decFilePath)
	assert.NoError(t, err)
}

func testEncryptFilenamePGP(t *testing.T) {
	filename := "test_large_file.bin"

	t.Logf("Filename: %s", filename)

	password := "test_password"

	encryptManager := NewEncryptionManager(EncryptionModePGP, false, []byte(password))

	encFilename, err := encryptManager.EncryptFilename(filename)
	assert.NoError(t, err)

	t.Logf("Encrypted filename: %s", encFilename)

	decFilename, err := encryptManager.DecryptFilename(encFilename)
	assert.NoError(t, err)

	t.Logf("Decrypted filename: %s", decFilename)

	// compare
	assert.Equal(t, filename, decFilename)
}

func testDecryptFilenameWinSCP(t *testing.T) {
	filename := "fVten7j3mxzA0LVfDcLSkycYrFHQqEU.aesctr.enc"

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encryptManager := NewEncryptionManager(EncryptionModeWinSCP, true, passwordBytes)

	decFilename, err := encryptManager.decryptFilenameWinSCP(filename)
	assert.NoError(t, err)

	assert.Equal(t, "LICENSE", decFilename)
}

func testEncryptFilenameWinSCP(t *testing.T) {
	filename := "LICENSE"

	password := "4444444444444444444444444444444444444444444444444444444444444444"
	passwordBytes, err := hex.DecodeString(password)
	assert.NoError(t, err)
	assert.Equal(t, len(passwordBytes), 32)

	encryptManager := NewEncryptionManager(EncryptionModeWinSCP, true, []byte(passwordBytes))

	encFilename, err := encryptManager.EncryptFilename(filename)
	assert.NoError(t, err)

	decFilename, err := encryptManager.DecryptFilename(encFilename)
	assert.NoError(t, err)

	// compare
	assert.Equal(t, filename, decFilename)
}
