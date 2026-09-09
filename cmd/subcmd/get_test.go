package subcmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cyverse/gocommands/cmd/flag"
)

func TestDecryptFileRemovesTemporaryFileAfterFailure(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), "download.aesctr.enc")
	if err := os.WriteFile(tempPath, []byte("not encrypted"), 0600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	get := &GetCommand{decryptionFlagValues: &flag.DecryptionFlagValues{}}
	if _, err := get.decryptFile("source.aesctr.enc", tempPath, filepath.Join(t.TempDir(), "output")); err == nil {
		t.Fatal("decryptFile() error = nil, want decryption error")
	}
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Errorf("temporary file still exists or stat failed: %v", err)
	}
}
