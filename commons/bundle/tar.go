package bundle

import (
	"archive/tar"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

type TarTrackerCallBack func(processed int64, total int64)

type TarEntry struct {
	sourcePath string
	sourceStat os.FileInfo // source file info
	targetPath string      // target path in a TAR file
}

type Tar struct {
	entries   []TarEntry
	totalSize int64 // total size of all files to be added to the tarball
}

func NewTar() *Tar {
	return &Tar{
		entries:   []TarEntry{},
		totalSize: 0,
	}
}

func (t *Tar) GetSize() int64 {
	return t.totalSize
}

func (t *Tar) AddEntry(sourcePath string, targetPath string) error {
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to get absolute path of %q: %w", sourcePath, err)
	}

	sourceStat, err := os.Stat(absSourcePath)
	if err != nil {
		if os.IsNotExist(err) {
			return irodsclient_types.NewFileNotFoundError(absSourcePath)
		}

		return xerrors.Errorf("failed to stat %q: %w", absSourcePath, err)
	}

	if sourceStat.IsDir() {
		return xerrors.Errorf("cannot add directory %q to tarball, only files are supported", absSourcePath)
	}

	entry := TarEntry{
		sourcePath: absSourcePath,
		sourceStat: sourceStat,
		targetPath: targetPath,
	}

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
		return xerrors.Errorf("failed to create a tarball file %q: %w", targetPath, err)
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
					return xerrors.Errorf("failed to stat directory %q: %w", localDirPath, statErr)
				}

				header, err := tar.FileInfoHeader(dirStat, dirStat.Name())
				if err != nil {
					return xerrors.Errorf("failed to create tar file info header for directory %s: %w", dirPath, err)
				}

				header.Name = strings.TrimSuffix(dirPath, "/") + "/"

				err = tarWriter.WriteHeader(header)
				if err != nil {
					return xerrors.Errorf("failed to write tar header for directory %s: %w", dirPath, err)
				}

				dirCreated[dirPath] = true
			}
		}

		header, err := tar.FileInfoHeader(entry.sourceStat, entry.sourceStat.Name())
		if err != nil {
			return xerrors.Errorf("failed to create tar file info header: %w", err)
		}

		header.Name = entry.targetPath

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return xerrors.Errorf("failed to write tar header: %w", err)
		}

		// add file content
		file, err := os.Open(entry.sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to open tar file %q: %w", entry.sourcePath, err)
		}

		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		if err != nil {
			return xerrors.Errorf("failed to write tar file: %w", err)
		}

		currentSize += entry.sourceStat.Size()

		if callback != nil {
			callback(currentSize, t.totalSize)
		}
	}

	return nil
}
