package commons

import (
	"os"
	"path/filepath"
	"strings"
)

func MakeIRODSPath(cwd string, path string) string {
	if strings.HasPrefix(path, "/") {
		return filepath.Clean(path)
	}

	newPath := filepath.Join(cwd, path)
	return filepath.Clean(newPath)
}

func MakeLocalPath(path string) string {
	if strings.HasPrefix(path, "/") {
		return filepath.Clean(path)
	}

	wd, _ := os.Getwd()

	newPath := filepath.Join(wd, path)
	return filepath.Clean(newPath)
}
