package types

import (
	"fmt"

	"github.com/cockroachdb/errors"
)

type NotDirError struct {
	Path string
}

func NewNotDirError(dest string) error {
	return &NotDirError{
		Path: dest,
	}
}

// Error returns error message
func (err *NotDirError) Error() string {
	return fmt.Sprintf("path %q is not a directory", err.Path)
}

// Is tests type of error
func (err *NotDirError) Is(other error) bool {
	_, ok := other.(*NotDirError)
	return ok
}

// ToString stringifies the object
func (err *NotDirError) ToString() string {
	return fmt.Sprintf("NotDirError: %q", err.Path)
}

// IsNotDirError evaluates if the given error is NotDirError
func IsNotDirError(err error) bool {
	var notDirErr *NotDirError
	return errors.As(err, &notDirErr)
}

type NotFileError struct {
	Path string
}

func NewNotFileError(dest string) error {
	return &NotFileError{
		Path: dest,
	}
}

// Error returns error message
func (err *NotFileError) Error() string {
	return fmt.Sprintf("path %q is not a file", err.Path)
}

// Is tests type of error
func (err *NotFileError) Is(other error) bool {
	_, ok := other.(*NotFileError)
	return ok
}

// ToString stringifies the object
func (err *NotFileError) ToString() string {
	return fmt.Sprintf("NotFileError: %q", err.Path)
}

// IsNotFileError evaluates if the given error is NotFileError
func IsNotFileError(err error) bool {
	var notFileErr *NotFileError
	return errors.As(err, &notFileErr)
}

type WebDAVError struct {
	URL       string
	ErrorCode int
}

func NewWebDAVError(url string, errorCode int) error {
	return &WebDAVError{
		URL:       url,
		ErrorCode: errorCode,
	}
}

// Error returns error message
func (err *WebDAVError) Error() string {
	return fmt.Sprintf("failed to access %q, received %d error", err.URL, err.ErrorCode)
}

// Is tests type of error
func (err *WebDAVError) Is(other error) bool {
	_, ok := other.(*WebDAVError)
	return ok
}

// ToString stringifies the object
func (err *WebDAVError) ToString() string {
	return fmt.Sprintf("WebDAVError: %q (error %d)", err.URL, err.ErrorCode)
}

// IsWebDAVError evaluates if the given error is WebDAVError
func IsWebDAVError(err error) bool {
	var webDAVErr *WebDAVError
	return errors.As(err, &webDAVErr)
}
