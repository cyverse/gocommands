package webdav

import (
	"os"
	"path/filepath"
	"testing"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

func TestDownloadZeroSizeFileToDirectory(t *testing.T) {
	destinationDir := t.TempDir()
	client := &WebDAVClient{}
	sourceEntry := &irodsclient_fs.Entry{
		Path: "/zone/home/empty.txt",
		Size: 0,
	}

	result, err := client.DownloadFile(sourceEntry, destinationDir, "", false, nil)
	if err != nil {
		t.Fatalf("DownloadFile() error = %v", err)
	}

	localPath := filepath.Join(destinationDir, "empty.txt")
	info, err := os.Stat(localPath)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", localPath, err)
	}
	if info.Size() != 0 {
		t.Errorf("file size = %d, want 0", info.Size())
	}
	if result.LocalPath != localPath {
		t.Errorf("result local path = %q, want %q", result.LocalPath, localPath)
	}
}
