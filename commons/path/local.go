package path

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"
)

func MakeLocalPath(localPath string) string {
	absLocalPath, err := filepath.Abs(localPath)
	if err != nil {
		return filepath.Clean(localPath)
	}

	return filepath.Clean(absLocalPath)
}

func MakeLocalTargetFilePath(source string, target string) string {
	realTarget, err := ResolveLocalSymlink(target)
	if err != nil {
		return target
	}

	st, err := os.Stat(realTarget)
	if err == nil {
		if st.IsDir() {
			// make full file name for target
			// source may be a local file or an iRODS file
			filename := GetBasename(source)
			return filepath.Join(target, filename)
		}
	}
	return target
}

// GetLocalParentDirPaths returns all parent dirs
func GetLocalParentDirPaths(p string) []string {
	parents := []string{}

	if p == string(os.PathSeparator) {
		return parents
	}

	absPath, _ := filepath.Abs(p)
	if filepath.Dir(absPath) == absPath {
		return parents
	}

	curPath := absPath

	for len(curPath) > 0 {
		curDir := filepath.Dir(curPath)
		if curDir == curPath {
			// root
			break
		}

		if len(curDir) > 0 {
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

func GetLocalCommonRootDirPath(paths []string) (string, error) {
	absPaths := make([]string, len(paths))

	// get abs paths
	for idx, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", xerrors.Errorf("failed to compute absolute path for %q: %w", path, err)
		}
		absPaths[idx] = absPath
	}

	// find shortest path
	commonRoot := getLocalCommonPrefix(absPaths...)

	commonRootStat, err := os.Stat(commonRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return "", irodsclient_types.NewFileNotFoundError(commonRoot)
		}

		return "", xerrors.Errorf("failed to stat %q: %w", commonRoot, err)
	}

	if commonRootStat.IsDir() {
		return commonRoot, nil
	}
	return filepath.Dir(commonRoot), nil
}

func getLocalCommonPrefix(paths ...string) string {
	// Handle special cases.
	if len(paths) == 0 {
		return ""
	} else if len(paths) == 1 {
		return filepath.Clean(paths[0])
	}

	c := []byte(filepath.Clean(paths[0]))
	c = append(c, filepath.Separator)

	// Ignore the first path since it's already in c
	for _, v := range paths[1:] {
		// Clear each path before testing it
		v = filepath.Clean(v) + string(filepath.Separator)

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
		if c[i] == filepath.Separator {
			c = c[:i]
			break
		}
	}

	return string(c)
}

func ExpandLocalHomeDirPath(p string) (string, error) {
	// resolve "~/"
	if p == "~" {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home directory: %w", err)
		}

		return filepath.Abs(homedir)
	} else if strings.HasPrefix(p, "~/") {
		homedir, err := os.UserHomeDir()
		if err != nil {
			return "", xerrors.Errorf("failed to get user home directory: %w", err)
		}

		p = filepath.Join(homedir, p[2:])
		return filepath.Abs(p)
	}

	return filepath.Abs(p)
}

func MarkLocalPathMap(pathMap map[string]bool, p string) {
	dirs := GetLocalParentDirPaths(p)

	for _, dir := range dirs {
		pathMap[dir] = true
	}

	pathMap[p] = true
}

func ResolveLocalSymlink(p string) (string, error) {
	st, err := os.Lstat(p)
	if err != nil {
		return "", xerrors.Errorf("failed to lstat path %q: %w", p, err)
	}

	if st.Mode()&os.ModeSymlink == os.ModeSymlink {
		// symlink
		new_p, err := filepath.EvalSymlinks(p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", p, err)
		}

		// follow recursively
		new_pp, err := ResolveLocalSymlink(new_p)
		if err != nil {
			return "", xerrors.Errorf("failed to evaluate symlink path %q: %w", new_p, err)
		}

		return new_pp, nil
	}
	return p, nil
}

/*
func ExistFile(p string) bool {
	realPath, err := ResolveSymlink(p)
	if err != nil {
		return false
	}

	st, err := os.Stat(realPath)
	if err != nil {
		return false
	}

	if !st.IsDir() {
		return true
	}
	return false
}
*/
