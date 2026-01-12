package bundle

import (
	"archive/tar"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	log "github.com/sirupsen/logrus"
)

type TarTrackerCallBack func(processed int64, total int64)

type TarEntry struct {
	sourcePath string
	sourceStat os.FileInfo // source file info
	targetPath string      // target path in a TAR file
}

type Tar struct {
	targetPath string
	entries    []TarEntry
	totalSize  int64 // total size of all files to be added to the tarball
}

func NewTar(targetPath string) *Tar {
	return &Tar{
		targetPath: targetPath,
		entries:    []TarEntry{},
		totalSize:  0,
	}
}

func (t *Tar) GetSize() int64 {
	return t.totalSize
}

func (t *Tar) AddEntry(sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"source_path":      sourcePath,
		"target_path":      targetPath,
		"target_root_path": t.targetPath,
	})

	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to get absolute path of %q", sourcePath)
	}

	sourceStat, err := os.Stat(absSourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(absSourcePath)
		}

		return errors.Wrapf(err, "failed to stat %q", absSourcePath)
	}

	if sourceStat.IsDir() {
		return errors.Wrapf(err, "cannot add directory %q to tarball, only files are supported", absSourcePath)
	}

	// use relative path for the target path in the tarball
	targetRelPath := targetPath
	if strings.HasPrefix(targetPath, t.targetPath) {
		targetRelPath = targetPath[len(t.targetPath):]
		targetRelPath = strings.TrimLeft(targetRelPath, "/")
	}

	entry := TarEntry{
		sourcePath: absSourcePath,
		sourceStat: sourceStat,
		targetPath: targetRelPath,
	}

	logger.Debugf("Adding %q as %q to tarball", absSourcePath, targetRelPath)

	t.entries = append(t.entries, entry)
	t.totalSize += sourceStat.Size()
	return nil
}

func (t *Tar) CreateTarball(targetPath string, callback TarTrackerCallBack) error {
	currentSize := int64(0)
	if callback != nil {
		callback(0, t.totalSize)
	}

	tarfile, err := os.Create(targetPath)
	if err != nil {
		return errors.Wrapf(err, "failed to create a tarball file %q", targetPath)
	}
	defer tarfile.Close()

	tarWriter := tar.NewWriter(tarfile)
	defer tarWriter.Close()

	dirCreated := map[string]bool{}

	for _, entry := range t.entries {
		dirPath := path.Dir(entry.targetPath)
		if dirPath != "." && dirPath != "/" {
			// has parent directory, create it if not exists
			if _, exists := dirCreated[dirPath]; !exists {
				// not created yet, create the directory in the tar
				localDirPath := filepath.Dir(entry.sourcePath)
				dirStat, statErr := os.Stat(localDirPath)
				if statErr != nil {
					return errors.Wrapf(statErr, "failed to stat directory %q", localDirPath)
				}

				header, err := tar.FileInfoHeader(dirStat, dirStat.Name())
				if err != nil {
					return errors.Wrapf(err, "failed to create tar file info header for directory %s", dirPath)
				}

				header.Name = strings.TrimSuffix(dirPath, "/") + "/"

				err = tarWriter.WriteHeader(header)
				if err != nil {
					return errors.Wrapf(err, "failed to write tar header for directory %s", dirPath)
				}

				dirCreated[dirPath] = true
			}
		}

		header, err := tar.FileInfoHeader(entry.sourceStat, entry.sourceStat.Name())
		if err != nil {
			return errors.Wrapf(err, "failed to create tar file info header")
		}

		header.Name = entry.targetPath

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return errors.Wrapf(err, "failed to write tar header")
		}

		// add file content
		file, err := os.Open(entry.sourcePath)
		if err != nil {
			return errors.Wrapf(err, "failed to open tar file %q", entry.sourcePath)
		}

		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return errors.Wrapf(err, "failed to write tar file %q", entry.sourcePath)
		}

		currentSize += entry.sourceStat.Size()

		if callback != nil {
			callback(currentSize, t.totalSize)
		}
	}

	return nil
}
