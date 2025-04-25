package commons

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
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
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "Tar",
	})

	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return xerrors.Errorf("failed to get absolute path of %q: %w", baseDir, err)
	}

	entries := []*TarEntry{}

	createdDirs := map[string]bool{}

	for _, source := range sources {
		source, err := filepath.Abs(source)
		if err != nil {
			return xerrors.Errorf("failed to get absolute path of %q: %w", source, err)
		}

		logger.Infof("adding a source %q (base %q) to a tarball %q", source, baseDir, target)

		sourceStat, err := os.Stat(source)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(source)
			}

			return xerrors.Errorf("failed to stat %q: %w", source, err)
		}

		rel, err := filepath.Rel(baseDir, source)
		if err != nil {
			return xerrors.Errorf("failed to compute relative path %q to %q: %w", source, baseDir, err)
		}

		parentDirs := GetParentLocalDirs(source)
		logger.Debugf("parent dirs %q", parentDirs)

		for _, pdir := range parentDirs {
			if _, ok := createdDirs[pdir]; !ok {
				baseDirPrefix := strings.TrimSuffix(baseDir, string(os.PathSeparator)) + string(os.PathSeparator)
				if strings.HasPrefix(pdir, baseDirPrefix) {
					// pdir is a subdir of baseDir
					// make entries for dir
					relDir, err := filepath.Rel(baseDir, pdir)
					if err != nil {
						return xerrors.Errorf("failed to compute relative path %q to %q: %w", pdir, baseDir, err)
					}

					entry := NewTarEntry(pdir, filepath.ToSlash(relDir))
					entries = append(entries, entry)
					logger.Infof("added a dir %q ==> %q in a tarball %q", entry.source, entry.target, target)
				}

				createdDirs[pdir] = true
			}
		}

		// make entries for file
		entry := NewTarEntry(source, filepath.ToSlash(rel))

		entries = append(entries, entry)
		logger.Infof("added a source %q ==> %q in a tarball %q", entry.source, entry.target, target)

		if sourceStat.IsDir() {
			createdDirs[rel] = true
		}
	}

	return makeTar(entries, target, callback)
}

func makeTar(entries []*TarEntry, target string, callback TrackerCallBack) error {
	totalSize := int64(0)
	currentSize := int64(0)
	for _, entry := range entries {
		sourceStat, err := os.Stat(entry.source)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(entry.source)
			}

			return xerrors.Errorf("failed to stat %q: %w", entry.source, err)
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
		return xerrors.Errorf("failed to create file %q: %w", target, err)
	}

	defer tarfile.Close()

	tarWriter := tar.NewWriter(tarfile)
	defer tarWriter.Close()

	for _, entry := range entries {
		sourceStat, err := os.Stat(entry.source)
		if err != nil {
			if os.IsNotExist(err) {
				return irodsclient_types.NewFileNotFoundError(entry.source)
			}

			return xerrors.Errorf("failed to stat %q: %w", entry.source, err)
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
				return xerrors.Errorf("failed to open tar file %q: %w", entry.source, err)
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
