package commons

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
)

func MakeIRODSPath(cwd string, homedir string, zone string, irodsPath string) string {
	if strings.HasPrefix(irodsPath, fmt.Sprintf("/%s/~", zone)) {
		// compat to icommands
		// relative path from user's home
		partLen := 3 + len(zone)
		newPath := path.Join(homedir, irodsPath[partLen:])
		return path.Clean(newPath)
	}

	if strings.HasPrefix(irodsPath, "/") {
		// absolute path
		return path.Clean(irodsPath)
	}

	if strings.HasPrefix(irodsPath, "~") {
		// relative path from user's home
		newPath := path.Join(homedir, irodsPath[1:])
		return path.Clean(newPath)
	}

	// relative path from current woring dir
	newPath := path.Join(cwd, irodsPath)
	return path.Clean(newPath)
}

func MakeLocalPath(localPath string) string {
	if strings.HasPrefix(localPath, string(os.PathSeparator)) {
		return filepath.Clean(localPath)
	}

	wd, _ := os.Getwd()

	newPath := filepath.Join(wd, localPath)
	return filepath.Clean(newPath)
}

func EnsureTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

func EnsureTargetLocalFilePath(source string, target string) string {
	st, err := os.Stat(target)
	if err == nil {
		if st.IsDir() {
			// make full file name for target
			filename := GetBasename(source)
			return filepath.Join(target, filename)
		}
	}
	return target
}

func GetFileExtension(path string) string {
	base := GetBasename(path)

	idx := strings.Index(base, ".")
	if idx >= 0 {
		return path[idx:]
	}
	return path
}

func GetBasename(path string) string {
	idx1 := strings.LastIndex(path, string(os.PathSeparator))
	idx2 := strings.LastIndex(path, "/")

	if idx1 < 0 && idx2 < 0 {
		return "."
	}

	if idx1 >= idx2 {
		return path[idx1+1:]
	}
	return path[idx2+1:]
}

func GetShortedLocalPath(paths []string) (string, error) {
	shortestPath := ""
	shortestPathDepth := 0

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}

		if len(shortestPath) == 0 {
			shortestPath = absPath
			shortestPathDepth = strings.Count(shortestPath, string(os.PathSeparator))
		} else {
			curDepth := strings.Count(absPath, string(os.PathSeparator))
			if shortestPathDepth > curDepth {
				shortestPath = absPath
				shortestPathDepth = curDepth
			}
		}
	}

	return shortestPath, nil
}
