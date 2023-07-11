package commons

import (
	"fmt"
	"path"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func ValidateStagingDir(fs *irodsclient_fs.FileSystem, targetPath string, stagingPath string) (bool, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "ValidateStagingDir",
	})

	stagingResourceServers, err := GetResourceServers(fs, stagingPath)
	if err != nil {
		return false, xerrors.Errorf("failed to get resource servers for %s: %w", stagingPath, err)
	}

	logger.Debugf("staging resource servers - %v", stagingResourceServers)

	targetResourceServers, err := GetResourceServers(fs, targetPath)
	if err != nil {
		return false, xerrors.Errorf("failed to get resource servers for %s: %w", targetPath, err)
	}

	logger.Debugf("target resource servers - %v", targetResourceServers)

	for _, trashResourceServer := range stagingResourceServers {
		for _, targetResourceServer := range targetResourceServers {
			if trashResourceServer == targetResourceServer {
				// same resource server
				return true, nil
			}
		}
	}

	return false, nil
}

func GetDefaultStagingDirInTargetPath(targetPath string) string {
	return path.Join(targetPath, ".gocmd_staging")
}

func IsStagingDirInTargetPath(stagingPath string) bool {
	return path.Base(stagingPath) == ".gocmd_staging"
}

func CheckSafeStagingDir(stagingPath string) error {
	dirParts := strings.Split(stagingPath[1:], "/")
	dirDepth := len(dirParts)

	if dirDepth < 3 {
		// no
		return xerrors.Errorf("staging path %s is not safe!", stagingPath)
	}

	// zone/home/user OR zone/home/shared (public)
	if dirParts[0] != GetZone() {
		return xerrors.Errorf("staging path %s is not safe, not in the correct zone", stagingPath)
	}

	if dirParts[1] != "home" {
		return xerrors.Errorf("staging path %s is not safe", stagingPath)
	}

	if dirParts[2] == GetUsername() {
		if dirDepth <= 3 {
			// /zone/home/user
			return xerrors.Errorf("staging path %s is not safe!", stagingPath)
		}
	} else {
		// public or shared?
		if dirDepth <= 4 {
			// /zone/home/public/dataset1
			return xerrors.Errorf("staging path %s is not safe!", stagingPath)
		}
	}

	return nil
}

func GetBundleFileName(managerID string, bundleIndex int64) string {
	return fmt.Sprintf("bundle_%s_%d.tar", managerID, bundleIndex)
}

func GetBundleFileNameParts(name string) (bool, string, string) {
	if !strings.HasSuffix(name, ".tar") {
		return false, "", ""
	}

	name = name[:len(name)-4]

	parts := strings.Split(name, "_")
	if len(parts) != 3 {
		return false, "", ""
	}

	return true, parts[1], parts[2]
}

func GetDefaultStagingDir(targetPath string) string {
	return GetDefaultStagingDirInTargetPath(targetPath)
}

func GetResourceServers(fs *irodsclient_fs.FileSystem, targetDir string) ([]string, error) {
	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return nil, xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	dirCreated := false
	if !fs.ExistsDir(targetDir) {
		err := fs.MakeDir(targetDir, true)
		if err != nil {
			return nil, xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
		}
		dirCreated = true
	}

	// write a new temp file and check resource server info
	testFilePath := path.Join(targetDir, "staging_test.txt")

	filehandle, err := fs.CreateFile(testFilePath, "", "w+")
	if err != nil {
		return nil, xerrors.Errorf("failed to create file %s: %w", testFilePath, err)
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
	collection, err := irodsclient_irodsfs.GetCollection(connection, targetDir)
	if err != nil {
		return nil, xerrors.Errorf("failed to get collection %s: %w", targetDir, err)
	}

	entry, err := irodsclient_irodsfs.GetDataObject(connection, collection, path.Base(testFilePath))
	if err != nil {
		return nil, xerrors.Errorf("failed to get data-object %s: %w", testFilePath, err)
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
