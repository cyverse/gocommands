package webdav

import (
	"io"
)

type ProgressCallback func(writeSize int)

type WriterWithProgress struct {
	baseWriter io.WriteCloser
	callback   ProgressCallback
}

func NewWriterWithProgress(baseWriter io.WriteCloser, callback ProgressCallback) *WriterWithProgress {
	return &WriterWithProgress{
		baseWriter: baseWriter,
		callback:   callback,
	}
}

func (w *WriterWithProgress) Write(p []byte) (n int, err error) {
	n, err = w.baseWriter.Write(p)
	if n > 0 {
		w.callback(n)
	}
	return n, err
}

func (w *WriterWithProgress) Close() error {
	return w.baseWriter.Close()
}

type ReaderWithProgress struct {
	baseReader io.ReadCloser
	callback   ProgressCallback
}

func NewReaderWithProgress(baseReader io.ReadCloser, callback ProgressCallback) *ReaderWithProgress {
	return &ReaderWithProgress{
		baseReader: baseReader,
		callback:   callback,
	}
}

func (r *ReaderWithProgress) Read(p []byte) (n int, err error) {
	n, err = r.baseReader.Read(p)
	if n > 0 {
		r.callback(n)
	}
	return n, err
}

func (r *ReaderWithProgress) Close() error {
	return r.baseReader.Close()
}
