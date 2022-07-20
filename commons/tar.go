package commons

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Tar(sources []string, target string) error {
	tarfile, err := os.Create(target)
	if err != nil {
		return err
	}

	defer tarfile.Close()

	tarWriter := tar.NewWriter(tarfile)
	defer tarWriter.Close()

	for _, source := range sources {
		sourceStat, err := os.Stat(source)
		if err != nil {
			return nil
		}

		var baseDir string
		if sourceStat.IsDir() {
			baseDir = filepath.Base(source)
		}

		err = filepath.Walk(source,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				header, err := tar.FileInfoHeader(info, info.Name())
				if err != nil {
					return err
				}

				if baseDir != "" {
					header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
				}

				err = tarWriter.WriteHeader(header)
				if err != nil {
					return err
				}

				if info.IsDir() {
					return nil
				}

				file, err := os.Open(path)
				if err != nil {
					return err
				}

				defer file.Close()

				_, err = io.Copy(tarWriter, file)
				return err
			},
		)

		if err != nil {
			return err
		}
	}

	return nil
}
