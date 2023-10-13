package commons

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"golang.org/x/xerrors"
)

func MakeIRODSPath(cwd string, homedir string, zone string, irodsPath string) string {
	irodsPath = strings.TrimPrefix(irodsPath, "i:")

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

func GetFileExtension(p string) string {
	base := GetBasename(p)

	idx := strings.Index(base, ".")
	if idx >= 0 {
		return p[idx:]
	}
	return p
}

func GetBasename(p string) string {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return p
	}

	if idx1 >= idx2 {
		return p[idx1+1:]
	}
	return p[idx2+1:]
}

func GetDir(p string) string {
	idx1 := strings.LastIndex(p, string(os.PathSeparator))
	idx2 := strings.LastIndex(p, "/")

	if idx1 < 0 && idx2 < 0 {
		return "/"
	}

	if idx1 >= idx2 {
		return p[:idx1]
	}
	return p[:idx2]
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

func commonPrefix(sep byte, paths ...string) string {
	// Handle special cases.
	switch len(paths) {
	case 0:
		return ""
	case 1:
		return path.Clean(paths[0])
	}

	c := []byte(path.Clean(paths[0]))
	c = append(c, sep)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clean up each path before testing it
		v = path.Clean(v) + string(sep)

		// Find the first non-common byte and truncate c
		if len(v) < len(c) {
			c = c[:len(v)]
		}
		for i := 0; i < len(c); i++ {
			if v[i] != c[i] {
				c = c[:i]
				break
			}
		}
	}

	// Remove trailing non-separator characters and the final separator
	for i := len(c) - 1; i >= 0; i-- {
		if c[i] == sep {
			c = c[:i]
			break
		}
	}

	return string(c)
}

func GetCommonRootLocalDirPath(paths []string) (string, error) {
	commonRootPath, err := GetCommonRootLocalDirPathForSync(paths)
	if err != nil {
		return "", err
	}

	if commonRootPath == "/" {
		return "/", nil
	}

	return filepath.Dir(commonRootPath), nil
}

func GetCommonRootLocalDirPathForSync(paths []string) (string, error) {
	absPaths := make([]string, len(paths))

	// get abs paths
	for idx, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", xerrors.Errorf("failed to compute absolute path for %s: %w", path, err)
		}
		absPaths[idx] = absPath
	}

	// find shortest path
	commonRoot := commonPrefix(filepath.Separator, absPaths...)
	commonRootStat, err := os.Stat(commonRoot)
	if err != nil {
		return "", xerrors.Errorf("failed to stat %s: %w", commonRoot, err)
	}

	if commonRootStat.IsDir() {
		return commonRoot, nil
	}
	return filepath.Dir(commonRoot), nil
}

func ExpandHomeDir(path string) (string, error) {
	// resolve "~/"
	if path == "~" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home dir: %w", err)
		}

		return homedir, nil
	} else if strings.HasPrefix(path, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home dir: %w", err)
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
