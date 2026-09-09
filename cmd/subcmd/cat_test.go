package subcmd

import (
	"errors"
	"testing"
)

type contentThenErrorReader struct {
	read bool
}

func (reader *contentThenErrorReader) Read(buf []byte) (int, error) {
	if reader.read {
		return 0, errors.New("read failed")
	}
	reader.read = true
	copy(buf, "partial content")
	return len("partial content"), errors.New("read failed")
}

func TestReadContentReturnsNonEOFError(t *testing.T) {
	reader := &contentThenErrorReader{}
	var content []byte

	err := readContent(reader, func(chunk []byte) {
		content = append(content, chunk...)
	})
	if err == nil || err.Error() != "read failed" {
		t.Fatalf("readContent() error = %v, want read failed", err)
	}
	if got := string(content); got != "partial content" {
		t.Errorf("displayed content = %q, want partial content", got)
	}
}
