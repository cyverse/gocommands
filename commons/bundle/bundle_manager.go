package bundle

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/encryption"
	commons_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

type BundleEntry struct {
	LocalPath      string
	TempPath       string
	IRODSPath      string
	Size           int64
	EncryptionMode encryption.EncryptionMode
}

type Bundle struct {
	manager *BundleManager

	index    int64
	entries  []BundleEntry
	irodsDir string
	size     int64

	sealed         bool   // indicates if the bundle is sealed and no more entries can be added
	bundleFilename string // populated when the bundle is sealed

	mutex sync.RWMutex
}

func NewBundle(manager *BundleManager, index int64) *Bundle {
	return &Bundle{
		manager:        manager,
		index:          index,
		entries:        []BundleEntry{},
		irodsDir:       "",
		size:           0,
		bundleFilename: "",
		sealed:         false,
		mutex:          sync.RWMutex{},
	}
}

func (bundle *Bundle) GetManager() *BundleManager {
	return bundle.manager
}

func (bundle *Bundle) GetID() int64 {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.index
}

func (bundle *Bundle) IsFull() bool {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.size >= bundle.manager.maxBundleFileSize || len(bundle.entries) >= bundle.manager.maxFileNumInBundle
}

func (bundle *Bundle) RequireTar() bool {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return len(bundle.entries) >= bundle.manager.minFileNumInBundle
}

func (bundle *Bundle) GetEntryNumber() int {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return len(bundle.entries)
}

func (bundle *Bundle) GetEntries() []BundleEntry {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.entries
}

func (bundle *Bundle) IsEmpty() bool {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return len(bundle.entries) == 0
}

func (bundle *Bundle) GetSize() int64 {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.size
}

func (bundle *Bundle) GetBundleFilename() string {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.bundleFilename
}

func (bundle *Bundle) GetIRODSDir() string {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.irodsDir
}

func (bundle *Bundle) makeBundleFilename() (string, error) {
	entryStrs := []string{}

	entryStrs = append(entryStrs, "empty_bundle")

	for _, entry := range bundle.entries {
		entryStrs = append(entryStrs, entry.LocalPath)
	}

	md5Hash := md5.New()

	for _, str := range entryStrs {
		_, err := md5Hash.Write([]byte(str))
		if err != nil {
			return "", xerrors.Errorf("failed to write: %w", err)
		}
	}

	sumBytes := md5Hash.Sum(nil)
	hexhash := hex.EncodeToString(sumBytes)
	return fmt.Sprintf("bundle_%s.tar", hexhash), nil
}

func (bundle *Bundle) IsSameDir(entry BundleEntry) bool {
	irodsDir := path.Dir(entry.IRODSPath)

	if len(bundle.irodsDir) == 0 {
		return true
	}

	if bundle.irodsDir == irodsDir {
		return true
	}

	return false
}

func (bundle *Bundle) Add(entry BundleEntry) error {
	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	if !bundle.IsSameDir(entry) {
		return xerrors.Errorf("cannot add entry %q to bundle: entries must be in the same directory %q", entry.IRODSPath, bundle.irodsDir)
	}

	bundle.entries = append(bundle.entries, entry)

	if len(bundle.irodsDir) == 0 {
		bundle.irodsDir = path.Dir(entry.IRODSPath)
	}

	bundle.size += entry.Size

	return nil
}

func (bundle *Bundle) Seal() error {
	bundle.mutex.Lock()
	defer bundle.mutex.Unlock()

	if bundle.sealed {
		// already sealed
		return xerrors.Errorf("bundle %d is already sealed", bundle.index)
	}

	filename, err := bundle.makeBundleFilename()
	if err != nil {
		return xerrors.Errorf("failed to make bundle filename: %w", err)
	}

	bundle.bundleFilename = filename

	bundle.sealed = true
	return nil
}

func (bundle *Bundle) IsSealed() bool {
	bundle.mutex.RLock()
	defer bundle.mutex.RUnlock()

	return bundle.sealed
}

type BundleManager struct {
	nextBundleIndex int64
	bundles         []*Bundle

	minFileNumInBundle int   // this determines if a bundle requires tar
	maxFileNumInBundle int   // this determines if a bundle is full
	maxBundleFileSize  int64 // this determines if a bundle is full

	localTempDirPath    string
	irodsStagingDirPath string

	mutex sync.RWMutex
}

func NewBundleManager(minFileNumInBundle int, maxFileNumInBundle int, maxBundleFileSize int64, localTempDirPath string, irodsStagingDirPath string) *BundleManager {
	return &BundleManager{
		nextBundleIndex: 0,
		bundles:         []*Bundle{},

		minFileNumInBundle: minFileNumInBundle,
		maxFileNumInBundle: maxFileNumInBundle,
		maxBundleFileSize:  maxBundleFileSize,

		localTempDirPath:    localTempDirPath,
		irodsStagingDirPath: irodsStagingDirPath,
	}
}

func (manager *BundleManager) getNextBundleIndex() int64 {
	idx := manager.nextBundleIndex
	manager.nextBundleIndex++
	return idx
}

func (manager *BundleManager) GetLocalTempDirPath() string {
	return manager.localTempDirPath
}

func (manager *BundleManager) GetIRODSStagingDirPath() string {
	return manager.irodsStagingDirPath
}

func (manager *BundleManager) Add(bundleEntry BundleEntry) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle",
		"struct":   "BundleManager",
		"function": "Add",
	})

	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if len(manager.bundles) == 0 {
		newBundle := NewBundle(manager, manager.getNextBundleIndex())
		manager.bundles = append(manager.bundles, newBundle)
	}

	currentBundle := manager.bundles[len(manager.bundles)-1]

	if currentBundle.IsFull() {
		logger.Debugf("last bundle %d is full, creating a new bundle", currentBundle.index)
		currentBundle.Seal()

		currentBundle = NewBundle(manager, manager.getNextBundleIndex())
		manager.bundles = append(manager.bundles, currentBundle)
	}

	if !currentBundle.IsSameDir(bundleEntry) {
		logger.Debugf("current bundle %d has different directory %q, creating a new bundle", currentBundle.index, currentBundle.irodsDir)
		currentBundle.Seal()

		currentBundle = NewBundle(manager, manager.getNextBundleIndex())
		manager.bundles = append(manager.bundles, currentBundle)
	}

	err := currentBundle.Add(bundleEntry)
	if err != nil {
		return xerrors.Errorf("failed to add local file %q to bundle %d: %w", bundleEntry.LocalPath, currentBundle.index, err)
	}

	logger.Debugf("added a local file %q to a bundle %d", bundleEntry.LocalPath, currentBundle.index)
	return nil
}

func (manager *BundleManager) GetBundles() []*Bundle {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()

	return manager.bundles
}

func (manager *BundleManager) DoneScheduling() {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()

	if len(manager.bundles) == 0 {
		return // no bundles to seal
	}

	currentBundle := manager.bundles[len(manager.bundles)-1]
	if currentBundle.IsSealed() {
		return // already sealed, no need to seal again
	}

	currentBundle.Seal()
}

func (manager *BundleManager) IsBundleFilename(p string) bool {
	if strings.HasPrefix(p, "bundle_") && strings.HasPrefix(p, ".tar") {
		return true
	}
	return false
}

func (manager *BundleManager) ClearLocalBundles() error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle",
		"struct":   "BundleManager",
		"function": "ClearLocalBundles",
	})

	logger.Debugf("clearing local bundle files in %q", manager.localTempDirPath)

	entries, err := os.ReadDir(manager.localTempDirPath)
	if err != nil {
		return xerrors.Errorf("failed to read a local temp directory %q: %w", manager.localTempDirPath, err)
	}

	bundleEntries := []string{}
	for _, entry := range entries {
		// filter only bundle files
		if manager.IsBundleFilename(entry.Name()) {
			fullPath := filepath.Join(manager.localTempDirPath, entry.Name())
			bundleEntries = append(bundleEntries, fullPath)
		}
	}

	deletedCount := 0
	for _, entry := range bundleEntries {
		logger.Debugf("deleting local bundle %q", entry)
		removeErr := os.Remove(entry)
		if removeErr != nil {
			return xerrors.Errorf("failed to remove old local bundle %q: %w", entry, removeErr)
		}
	}

	terminal.Printf("deleted %d of %d local bundles in %q\n", deletedCount, len(bundleEntries), manager.localTempDirPath)
	logger.Debugf("deleted %d of %d local bundles in %q", deletedCount, len(bundleEntries), manager.localTempDirPath)

	return nil
}

func (manager *BundleManager) ClearIRODSBundles(fs *irodsclient_fs.FileSystem, removeDir bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "bundle",
		"struct":   "BundleManager",
		"function": "ClearIRODSBundles",
	})

	logger.Debugf("clearing irods bundle files in %q", manager.irodsStagingDirPath)

	if !fs.ExistsDir(manager.irodsStagingDirPath) {
		return xerrors.Errorf("staging dir %q does not exist", manager.irodsStagingDirPath)
	}

	entries, err := fs.List(manager.irodsStagingDirPath)
	if err != nil {
		return xerrors.Errorf("failed to list %q: %w", manager.irodsStagingDirPath, err)
	}

	deletedCount := 0
	for _, entry := range entries {
		// filter only bundle files
		if entry.Type == irodsclient_fs.FileEntry {
			if manager.IsBundleFilename(entry.Name) {
				logger.Debugf("deleting irods bundle %q", entry.Path)
				removeErr := fs.RemoveFile(entry.Path, true)
				if removeErr != nil {
					return xerrors.Errorf("failed to remove bundle file %q: %w", entry.Path, removeErr)
				} else {
					deletedCount++
				}
			}
		}
	}

	terminal.Printf("deleted %d of %d irods bundles in %q\n", deletedCount, len(entries), manager.irodsStagingDirPath)
	logger.Debugf("deleted %d of %d irods bundles in %q", deletedCount, len(entries), manager.irodsStagingDirPath)

	if removeDir {
		if len(entries) != deletedCount {
			// not all entries are deleted, so we do not remove the directory
			logger.Debugf("not all entries are deleted, not removing the staging directory %q", manager.irodsStagingDirPath)
			return nil
		}

		rmdirErr := fs.RemoveDir(manager.irodsStagingDirPath, true, true)
		if rmdirErr != nil {
			return xerrors.Errorf("failed to remove staging directory %q: %w", manager.irodsStagingDirPath, rmdirErr)
		}
	}

	return nil
}

func GetStagingDirInTargetPath(fs *irodsclient_fs.FileSystem, targetPath string) string {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	account := fs.GetAccount()
	zone := account.ClientZone
	stagingPath := path.Join(targetPath, ".gocmd_staging")
	return commons_path.MakeIRODSPath(cwd, home, zone, stagingPath)
}

func EnsureStagingDirPath(fs *irodsclient_fs.FileSystem, stagingPath string) (bool, error) {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	account := fs.GetAccount()
	zone := account.ClientZone
	stagingPath = commons_path.MakeIRODSPath(cwd, home, zone, stagingPath)

	dirParts := strings.Split(stagingPath[1:], "/")
	dirDepth := len(dirParts)

	if dirDepth < 3 {
		// no
		return false, xerrors.Errorf("staging path %q is not safe!", stagingPath)
	}

	// zone/home/user OR zone/home/shared (public)
	if dirParts[0] != account.ClientZone {
		return false, xerrors.Errorf("staging path %q is not safe, not in the correct zone", stagingPath)
	}

	if dirParts[1] != "home" {
		return false, xerrors.Errorf("staging path %q is not safe", stagingPath)
	}

	if dirParts[2] == account.ClientUser {
		if dirDepth <= 3 {
			// /zone/home/user
			return false, xerrors.Errorf("staging path %q is not safe!", stagingPath)
		}
	} else {
		// public or shared?
		if dirDepth <= 4 {
			// /zone/home/public/dataset1
			return false, xerrors.Errorf("staging path %q is not safe!", stagingPath)
		}
	}

	// make dir if not exists
	if fs.ExistsDir(stagingPath) {
		// already exists
		return false, nil
	}

	mkdirErr := fs.MakeDir(stagingPath, true)
	if mkdirErr != nil {
		return false, xerrors.Errorf("failed to make staging directory %q: %w", stagingPath, mkdirErr)
	}

	return true, nil
}

///

func GetResourceServersForDir(fs *irodsclient_fs.FileSystem, targetDir string) ([]string, error) {
	connection, err := fs.GetMetadataConnection(true)
	if err != nil {
		return nil, xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	dirCreated := false
	if !fs.ExistsDir(targetDir) {
		err := fs.MakeDir(targetDir, true)
		if err != nil {
			return nil, xerrors.Errorf("failed to make a directory %q: %w", targetDir, err)
		}
		dirCreated = true
	}

	// write a new temp file and check resource server info
	testFilePath := path.Join(targetDir, "staging_test.txt")

	filehandle, err := fs.CreateFile(testFilePath, "", "w+")
	if err != nil {
		return nil, xerrors.Errorf("failed to create file %q: %w", testFilePath, err)
	}

	_, err = filehandle.Write([]byte("resource server test\n"))
	if err != nil {
		return nil, xerrors.Errorf("failed to write: %w", err)
	}

	err = filehandle.Close()
	if err != nil {
		return nil, xerrors.Errorf("failed to close file: %w", err)
	}

	// data object
	entry, err := irodsclient_irodsfs.GetDataObject(connection, testFilePath)
	if err != nil {
		return nil, xerrors.Errorf("failed to get data-object %q: %w", testFilePath, err)
	}

	resourceServers := []string{}
	for _, replica := range entry.Replicas {
		resourceNames := strings.Split(replica.ResourceHierarchy, ";")
		if len(resourceNames) > 0 {
			resourceServers = append(resourceServers, resourceNames[0])
		}
	}

	fs.RemoveFile(testFilePath, true)

	if dirCreated {
		fs.RemoveDir(targetDir, true, true)
	}

	return resourceServers, nil
}

func IsSameResourceServer(fs *irodsclient_fs.FileSystem, path1 string, path2 string) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "bundle_manager",
		"function": "IsSameResourceServer",
	})

	path1RS, err := GetResourceServersForDir(fs, path1)
	if err != nil {
		return false, xerrors.Errorf("failed to get resource servers for %q: %w", path1, err)
	}

	logger.Debugf("resource servers for path %q - %v", path1, path1RS)

	path2RS, err := GetResourceServersForDir(fs, path2)
	if err != nil {
		return false, xerrors.Errorf("failed to get resource servers for %q: %w", path2, err)
	}

	logger.Debugf("staging resource servers for path %q - %v", path2, path2RS)

	for _, stagingResourceServer := range path2RS {
		for _, targetResourceServer := range path1RS {
			if stagingResourceServer == targetResourceServer {
				// same resource server
				return true, nil
			}
		}
	}

	return false, nil
}
