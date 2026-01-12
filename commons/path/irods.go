package path

import (
	"fmt"
	"path"
	"sort"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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

func MakeIRODSTargetFilePath(filesystem *irodsclient_fs.FileSystem, source string, target string) string {
	if filesystem.ExistsDir(target) {
		// make full file name for target
		// source may be a local file or an iRODS file
		filename := GetBasename(source)
		return path.Join(target, filename)
	}
	return target
}

// GetIRODSParentDirPaths returns all parent dirs
func GetIRODSParentDirPaths(p string) []string {
	parents := []string{}

	if p == "/" {
		return parents
	}

	curPath := p
	for len(curPath) > 0 && curPath != "/" {
		curDir := path.Dir(curPath)
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

func MarkIRODSPathMap(pathMap map[string]bool, p string) {
	dirs := GetIRODSParentDirPaths(p)

	for _, dir := range dirs {
		pathMap[dir] = true
	}

	pathMap[p] = true
}
