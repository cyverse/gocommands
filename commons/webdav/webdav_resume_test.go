package webdav

import (
	"crypto/md5"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/studio-b12/gowebdav"
)

func TestDownloadFileVerifiesAndRestartsAfterPartialDownloadMismatch(t *testing.T) {
	const content = "abcdef"
	var receivedRanges []string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedRanges = append(receivedRanges, request.Header.Get("Range"))
		_, _ = writer.Write([]byte(content))
	}))
	defer server.Close()

	localPath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(localPath, []byte("BAD"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	client := &WebDAVClient{baseURL: server.URL, webdav: gowebdav.NewClient(server.URL, "", "")}
	checksum := md5.Sum([]byte(content))
	entry := &irodsclient_fs.Entry{Path: "/zone/file", Size: int64(len(content)), CheckSum: checksum[:], CheckSumAlgorithm: "MD5"}

	if _, err := client.DownloadFile(entry, localPath, "", false, nil); err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != content {
		t.Errorf("downloaded content = %q, want %q", got, content)
	}
	if len(receivedRanges) < 2 || receivedRanges[0] != "bytes=3-5" || receivedRanges[len(receivedRanges)-1] != "bytes=0-5" {
		t.Errorf("Range headers = %q, want a partial request followed by a full restart", receivedRanges)
	}
}

func TestDownloadFilePreservesPartialFileAfterFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusPartialContent)
		_, _ = writer.Write([]byte("abc"))
	}))
	defer server.Close()

	localPath := filepath.Join(t.TempDir(), "file")
	client := &WebDAVClient{baseURL: server.URL, webdav: gowebdav.NewClient(server.URL, "", "")}
	entry := &irodsclient_fs.Entry{Path: "/zone/file", Size: 6}

	if _, err := client.DownloadFile(entry, localPath, "", false, nil); err == nil {
		t.Fatal("DownloadFile() succeeded with a truncated response")
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if got := string(data); got != "abc" {
		t.Errorf("partial file content = %q, want abc", got)
	}
}
