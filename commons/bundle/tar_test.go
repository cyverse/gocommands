package bundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTarAddEntryRejectsDirectory(t *testing.T) {
	tempDir := t.TempDir()
	tarball := NewTar("/target")

	if err := tarball.AddEntry(tempDir, "/target/dir"); err == nil {
		t.Fatal("AddEntry() succeeded for a directory")
	}
}

func TestCreateTarballRemovesPartialFileOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, "source")
	if err := os.WriteFile(sourcePath, []byte("content"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	tarball := NewTar("/target")
	if err := tarball.AddEntry(sourcePath, "/target/source"); err != nil {
		t.Fatalf("AddEntry() error = %v", err)
	}
	if err := os.Remove(sourcePath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	tarballPath := filepath.Join(tempDir, "bundle.tar")
	if err := tarball.CreateTarball(tarballPath, nil); err == nil {
		t.Fatal("CreateTarball() succeeded after its source file was removed")
	}
	if _, err := os.Stat(tarballPath); !os.IsNotExist(err) {
		t.Errorf("partial tarball still exists, stat error = %v", err)
	}
}
