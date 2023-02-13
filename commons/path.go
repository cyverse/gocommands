package commons

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

var (
	statCache = map[string]*irodsclient_fs.Entry{}
	dirCache  = map[string][]*irodsclient_fs.Entry{}

	cacheLock = sync.Mutex{}
)

func ListIRODSDir(filesystem *irodsclient_fs.FileSystem, irodsPath string) ([]*irodsclient_fs.Entry, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	if entries, ok := dirCache[irodsPath]; ok {
		return entries, nil
	}

	// no cache
	dirStat, err := filesystem.StatDir(irodsPath)
	if err != nil {
		return nil, err
	}

	statCache[irodsPath] = dirStat

	entries, err := filesystem.List(irodsPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		statCache[entry.Path] = entry
	}

	dirCache[irodsPath] = entries
	return entries, nil
}

func StatIRODSPath(filesystem *irodsclient_fs.FileSystem, irodsPath string) (*irodsclient_fs.Entry, error) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	dirParts := strings.Split(irodsPath[1:], "/")
	dirDepth := len(dirParts)

	// zone/home/user OR zone/home/shared (public)
	// don't scan parent
	if dirDepth <= 3 {
		if entry, ok := statCache[irodsPath]; ok {
			return entry, nil
		}

		entry, err := filesystem.Stat(irodsPath)
		if err != nil {
			return nil, err
		}

		statCache[irodsPath] = entry
		return entry, nil
	}

	if entry, ok := statCache[irodsPath]; ok {
		return entry, nil
	}

	// otherwise, list parent dir and cache all files in the dir
	parentDirPath := path.Dir(irodsPath)

	// no cache
	if _, ok := dirCache[parentDirPath]; !ok {
		parentDirStat, err := filesystem.StatDir(parentDirPath)
		if err != nil {
			return nil, err
		}

		statCache[parentDirPath] = parentDirStat

		entries, err := filesystem.List(parentDirPath)
		if err != nil {
			return nil, err
		}

		for _, entry := range entries {
			statCache[entry.Path] = entry
		}

		dirCache[parentDirPath] = entries
	}

	if entry, ok := statCache[irodsPath]; ok {
		return entry, nil
	}

	return nil, irodsclient_types.NewFileNotFoundError("could not find a data object or a directory")
}

func ExistsIRODSFile(filesystem *irodsclient_fs.FileSystem, irodsPath string) bool {
	entry, err := StatIRODSPath(filesystem, irodsPath)
	if err != nil {
		return false
	}

	if entry.Type == irodsclient_fs.FileEntry {
		return true
	}

	return false
}

func ExistsIRODSDir(filesystem *irodsclient_fs.FileSystem, irodsPath string) bool {
	entry, err := StatIRODSPath(filesystem, irodsPath)
	if err != nil {
		return false
	}

	if entry.Type == irodsclient_fs.DirectoryEntry {
		return true
	}

	return false
}

func ExistsIRODSPath(filesystem *irodsclient_fs.FileSystem, irodsPath string) bool {
	entry, err := StatIRODSPath(filesystem, irodsPath)
	if err != nil {
		return false
	}

	if entry.ID > 0 {
		return true
	}

	return false
}

func ClearIRODSDirCache(filesystem *irodsclient_fs.FileSystem, irodsPath string) {
	cacheLock.Lock()
	defer cacheLock.Unlock()

	delete(statCache, irodsPath)

	if entries, ok := dirCache[irodsPath]; ok {
		for _, entry := range entries {
			delete(statCache, entry.Path)
		}
		delete(dirCache, irodsPath)
	}

	if irodsPath != "/" {
		parentDirPath := path.Dir(irodsPath)
		if entries, ok := dirCache[parentDirPath]; ok {
			newEntries := []*irodsclient_fs.Entry{}

			for _, entry := range entries {
				if entry.Path != irodsPath {
					newEntries = append(newEntries, entry)
				}
			}

			dirCache[parentDirPath] = newEntries
		}
	}
}

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
