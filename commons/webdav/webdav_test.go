package webdav

import (
	"encoding/hex"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/stretchr/testify/assert"
)

func TestWebDAV(t *testing.T) {
	t.Run("test DownloadFileFromWebDAV", testDownloadFileFromWebDAV)
}

func testDownloadFileFromWebDAV(t *testing.T) {
	checksumBytes, _ := hex.DecodeString("713133e1a59ef6d1e42aa5405beae0de")
	sourceEntry := &irodsclient_fs.Entry{
		ID:                12345,
		Path:              "/iplant/home/iychoi/test_70MB.bin",
		Name:              "test_70MB.bin",
		Size:              71680000,
		CheckSum:          checksumBytes,
		CheckSumAlgorithm: "MD5",
	}

	localPath := "/tmp/test_70MB.bin"

	log.SetLevel(log.DebugLevel)

	callback := func(taskName string, progress int64, total int64) {
		// This is a dummy callback function
		t.Logf("Progress: %d/%d", progress, total)
	}

	webdav := NewWebDAVClient(nil, "https://data.cyverse.org/dav", "username", "password")

	transferResult, err := webdav.DownloadFile(sourceEntry, localPath, true, callback)
	assert.NoError(t, err)

	os.Remove(localPath) // Clean up the test file

	assert.Equal(t, sourceEntry.Path, transferResult.IRODSPath)
	t.Log("Transfer Result:", transferResult)
}
