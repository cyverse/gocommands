package commons

import (
	"path"
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

// list irods dir from cache, doesn't provide full cache consistency
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

// stat from cache, doesn't provide full cache consistency
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
