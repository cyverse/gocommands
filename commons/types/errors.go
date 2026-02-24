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
