package encryption

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDecryptFileWinSCPRejectsTruncatedSalt(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source")
	targetPath := filepath.Join(directory, "target")
	data := append([]byte(WinSCPAesCtrHeader), []byte("short salt")...)
	if err := os.WriteFile(sourcePath, data, 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := DecryptFileWinSCP(sourcePath, targetPath, make([]byte, 32)); err == nil {
		t.Fatal("DecryptFileWinSCP() error = nil, want truncated salt error")
	}
}
