package commons

import (
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

func GetDefaultStagingDir(fs *irodsclient_fs.FileSystem, targetPath string) (string, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "GetDefaultStagingDir",
	})

	targetStagingDirPath := GetDefaultStagingDirInTargetPath(targetPath)
	trashDirPath := GetTrashHomeDir()

	trashResourceServers, err := GetResourceServers(fs, trashDirPath)
	if err != nil {
		return "", xerrors.Errorf("failed to get resource servers for %s: %w", trashDirPath, err)
	}

	logger.Debugf("trash resource servers - %v", trashResourceServers)

	targetResourceServers, err := GetResourceServers(fs, targetStagingDirPath)
	if err != nil {
		return "", xerrors.Errorf("failed to get resource servers for %s: %w", targetStagingDirPath, err)
	}

	logger.Debugf("target resource servers - %v", targetResourceServers)

	for _, trashResourceServer := range trashResourceServers {
		for _, targetResourceServer := range targetResourceServers {
			if trashResourceServer == targetResourceServer {
				// same resource server
				return trashDirPath, nil
			}
		}
	}

	return targetStagingDirPath, nil
}

func GetResourceServers(fs *irodsclient_fs.FileSystem, targetDir string) ([]string, error) {
	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return nil, xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	if !fs.ExistsDir(targetDir) {
		err := fs.MakeDir(targetDir, true)
		if err != nil {
			return nil, xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
		}
	}

	// write a new temp file and check resource server info
	testFilePath := path.Join(targetDir, "staging_test.txt")

	filehandle, err := fs.CreateFile(testFilePath, "", "w")
	if err != nil {
		return nil, xerrors.Errorf("failed to crate file %s: %w", testFilePath, err)
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

	return resourceServers, nil
}
