package commons

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/xerrors"
)

type TrackerCallBack func(processed int64, total int64)

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

func Tar(baseDir string, sources []string, target string, callback TrackerCallBack) error {
	entries := []*TarEntry{}

	createdDirs := map[string]bool{}

	for _, source := range sources {
		sourceStat, err := os.Stat(source)
		if err != nil {
			return xerrors.Errorf("failed to stat %s: %w", source, err)
		}

		if sourceStat.IsDir() {
			// do not include dir for now
			continue
		}

		rel, err := filepath.Rel(baseDir, source)
		if err != nil {
			return xerrors.Errorf("failed to compute relative path %s to %s: %w", source, baseDir, err)
		}

		pdirs := GetParentLocalDirs(rel)
		for _, pdir := range pdirs {
			if _, ok := createdDirs[pdir]; !ok {
				// make entries for dir
				entry := NewTarEntry(filepath.Join(baseDir, pdir), filepath.ToSlash(pdir))
				entries = append(entries, entry)

				createdDirs[pdir] = true
			}
		}

		// make entries for file
		entry := NewTarEntry(source, filepath.ToSlash(rel))
		entries = append(entries, entry)
	}

	return makeTar(entries, target, callback)
}

func makeTar(entries []*TarEntry, target string, callback TrackerCallBack) error {
	totalSize := int64(0)
	currentSize := int64(0)
	for _, entry := range entries {
		sourceStat, err := os.Stat(entry.source)
		if err != nil {
			return xerrors.Errorf("failed to stat %s: %w", entry.source, err)
		}

		if !sourceStat.IsDir() {
			totalSize += sourceStat.Size()
		}
	}

	if callback != nil {
		callback(0, totalSize)
	}

	tarfile, err := os.Create(target)
	if err != nil {
		return xerrors.Errorf("failed to create file %s: %w", target, err)
	}

	defer tarfile.Close()

	tarWriter := tar.NewWriter(tarfile)
	defer tarWriter.Close()

	for _, entry := range entries {
		sourceStat, err := os.Stat(entry.source)
		if err != nil {
			return xerrors.Errorf("failed to stat %s: %w", entry.source, err)
		}

		header, err := tar.FileInfoHeader(sourceStat, sourceStat.Name())
		if err != nil {
			return xerrors.Errorf("failed to create tar file info header: %w", err)
		}

		header.Name = entry.target

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return xerrors.Errorf("failed to write tar header: %w", err)
		}

		if !sourceStat.IsDir() {
			// add file content
			file, err := os.Open(entry.source)
			if err != nil {
				return xerrors.Errorf("failed to open tar file %s: %w", entry.source, err)
			}

			defer file.Close()

			_, err = io.Copy(tarWriter, file)
			if err != nil {
				return xerrors.Errorf("failed to write tar file: %w", err)
			}

			currentSize += sourceStat.Size()

			if callback != nil {
				callback(currentSize, totalSize)
			}
		}
	}

	return nil
}
