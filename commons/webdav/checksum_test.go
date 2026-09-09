package webdav

import (
	"errors"
	"testing"
)

type recordingFileRemover struct {
	path  string
	force bool
	err   error
}

func (remover *recordingFileRemover) RemoveFile(path string, force bool) error {
	remover.path = path
	remover.force = force
	return remover.err
}

func TestValidateUploadedChecksum(t *testing.T) {
	t.Run("missing remote checksum does not remove file", func(t *testing.T) {
		remover := &recordingFileRemover{}
		err := validateUploadedChecksum(remover, "/zone/file", nil, []byte{1})
		if err != nil {
			t.Fatalf("validateUploadedChecksum() error = %v", err)
		}
		if remover.path != "" {
			t.Errorf("RemoveFile called for %q, want no call", remover.path)
		}
	})

	t.Run("mismatch removes remote file", func(t *testing.T) {
		remover := &recordingFileRemover{}
		err := validateUploadedChecksum(remover, "/zone/file", []byte{1}, []byte{2})
		if err == nil {
			t.Fatal("validateUploadedChecksum() succeeded for a mismatch")
		}
		if remover.path != "/zone/file" || !remover.force {
			t.Errorf("RemoveFile(%q, %t), want (/zone/file, true)", remover.path, remover.force)
		}
	})

	t.Run("cleanup failure is returned", func(t *testing.T) {
		remover := &recordingFileRemover{err: errors.New("remove failed")}
		err := validateUploadedChecksum(remover, "/zone/file", []byte{1}, []byte{2})
		if err == nil || err.Error() == "" {
			t.Fatal("validateUploadedChecksum() did not return the cleanup failure")
		}
	})

	t.Run("matching checksums succeed", func(t *testing.T) {
		remover := &recordingFileRemover{}
		if err := validateUploadedChecksum(remover, "/zone/file", []byte{1}, []byte{1}); err != nil {
			t.Fatalf("validateUploadedChecksum() error = %v", err)
		}
		if remover.path != "" {
			t.Errorf("RemoveFile called for %q, want no call", remover.path)
		}
	})
}
