package commons

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
)

type TarEntry struct {
	source string
	target string // target path in a TAR file
}

func NewTarEntry(source string, target string) *TarEntry {
	return &TarEntry{
		source: source,
		target: target,
	}
}

func Tar(baseDir string, sources []string, target string) error {
	entries := []*TarEntry{}

	for _, source := range sources {
		rel, err := filepath.Rel(baseDir, source)
		if err != nil {
			return err
		}

		// make entries
		entry := NewTarEntry(source, filepath.ToSlash(rel))
		entries = append(entries, entry)
	}

	return makeTar(entries, target)
}

func makeTar(entries []*TarEntry, target string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}

	defer tarfile.Close()

	tarWriter := tar.NewWriter(tarfile)
	defer tarWriter.Close()

	for _, entry := range entries {
		sourceStat, err := os.Stat(entry.source)
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(sourceStat, sourceStat.Name())
		if err != nil {
			return err
		}

		header.Name = entry.target

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return err
		}

		if !sourceStat.IsDir() {
			// add file content
			file, err := os.Open(entry.source)
			if err != nil {
				return err
			}

			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
