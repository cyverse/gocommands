package commons

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
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
	absLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return filepath.Clean(localPath)
	}

	return filepath.Clean(absLocalPath)
}

func MakeTargetIRODSFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

func MakeTargetLocalFilePath(source string, target string) string {
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

// GetParentLocalDirs returns all parent dirs
func GetParentLocalDirs(p string) []string {
	parents := []string{}

	if p == string(os.PathSeparator) || p == "." {
		return parents
	}

	curPath := p
	for len(curPath) > 0 && curPath != string(os.PathSeparator) && curPath != "." {
		curDir := filepath.Dir(curPath)
		if len(curDir) > 0 && curDir != "." {
			parents = append(parents, curDir)
		}

		curPath = curDir
	}

	// sort
	sort.Slice(parents, func(i int, j int) bool {
		return len(parents[i]) < len(parents[j])
	})

	return parents
}

func GetCommonRootLocalDirPath(paths []string) (string, error) {
	// find shortest path
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

	commonRootPath := shortestPath
	for {
		pass := true
		// check it with others
		for _, path := range paths {
			absPath, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}

			rel, err := filepath.Rel(commonRootPath, absPath)
			if err != nil {
				return "", err
			}

			if strings.HasPrefix(rel, "../") {
				commonRootPath = filepath.Dir(commonRootPath)
				pass = false
				break
			}
		}

		if pass {
			break
		}
	}

	if commonRootPath == "" {
		return "/", nil
	}

	return commonRootPath, nil
}

func ExpandHomeDir(path string) (string, error) {
	// resolve "~/"
	if path == "~" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		return homedir, nil
	} else if strings.HasPrefix(path, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}

		path = filepath.Join(homedir, path[2:])
		return filepath.Clean(path), nil
	}

	return path, nil
}

func ExistFile(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return true
	}
	return false
}
