package subcmd

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync i:[local dir] [collection] or sync [collection] i:[local dir]",
	Short: "Sync local directory with iRODS collection",
	Long:  `This synchronizes a local directory with the given iRODS collection.`,
	RunE:  processSyncCommand,
}

func AddSyncCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(syncCmd)

	rootCmd.AddCommand(syncCmd)
}

func processSyncCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processSyncCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		logger.Error(err)
	}

	if !cont {
		return err
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		return err
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return err
	}

	defer filesystem.Release()

	if len(args) >= 2 {
		targetPath := args[len(args)-1]
		for _, sourcePath := range args[:len(args)-1] {
			if strings.HasPrefix(sourcePath, "i:") {
				if strings.HasPrefix(targetPath, "i:") {
					// copy
					err = syncCopyOne(filesystem, sourcePath[2:], targetPath[2:])
					if err != nil {
						logger.Error(err)
						return err
					}
				} else {
					// get
					err = syncGetOne(filesystem, sourcePath[2:], targetPath)
					if err != nil {
						logger.Error(err)
						return err
					}
				}
			} else {
				if strings.HasPrefix(targetPath, "i:") {
					// put
					err = syncPutOne(filesystem, sourcePath, targetPath[2:])
					if err != nil {
						logger.Error(err)
						return err
					}
				} else {
					// local to local
					return fmt.Errorf("syncing between local files/directories is not supported")
				}
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}
	return nil
}

func syncGetOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncGetOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	entry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if entry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.EnsureTargetLocalFilePath(sourcePath, targetPath)

		st, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}

			logger.Debugf("there is no file %s at local", targetFilePath)
		} else {
			// file/dir exists
			if st.IsDir() {
				// dir
				logger.Debugf("local path %s is a directory, deleting", targetFilePath)
				err = os.RemoveAll(targetFilePath)
				if err != nil {
					return err
				}
			} else {
				// file
				md5hash, err := hashLocalFileMD5(targetFilePath)
				if err != nil {
					return err
				}

				if entry.CheckSum == md5hash && entry.Size == st.Size() {
					// match
					logger.Debugf("local file %s is up-to-date", targetFilePath)
					return nil
				}

				// delete first
				logger.Debugf("local file %s is has a different hash code, deleting", targetFilePath)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", entry.CheckSum, md5hash, entry.Size, st.Size())
				err = os.Remove(targetFilePath)
				if err != nil {
					return err
				}
			}
		}

		logger.Debugf("downloading a data object %s to %s", sourcePath, targetFilePath)
		err = filesystem.DownloadFileParallel(sourcePath, "", targetPath, 0)
		if err != nil {
			return err
		}
	} else {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(entry.Path)
		if err != nil {
			return err
		}

		// make target dir if not exists
		err = os.MkdirAll(targetPath, 0766)
		if err != nil {
			return err
		}

		for _, entryInDir := range entries {
			targetEntryPath := filepath.Join(targetPath, entryInDir.Name)
			err = syncGetOne(filesystem, entryInDir.Path, targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncPutOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncPutOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	st, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !st.IsDir() {
		targetFilePath := commons.EnsureTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		logger.Debugf("checking path %s existance", targetFilePath)
		if filesystem.Exists(targetFilePath) {
			logger.Debugf("path %s exists", targetFilePath)

			// already exists!
			if filesystem.ExistsDir(targetFilePath) {
				// dir
				logger.Debugf("path %s is a collection, deleting", targetFilePath)
				err = filesystem.RemoveDir(targetFilePath, true, true)
				if err != nil {
					return err
				}
			} else {
				// file
				entry, err := filesystem.Stat(targetFilePath)
				if err != nil {
					return err
				}

				md5hash, err := hashLocalFileMD5(sourcePath)
				if err != nil {
					return err
				}

				if entry.CheckSum == md5hash && entry.Size == st.Size() {
					// match
					logger.Debugf("data object %s is up-to-date", targetFilePath)
					return nil
				}

				// delete first
				logger.Debugf("data object %s is has a different hash code, deleting", targetFilePath)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", md5hash, entry.CheckSum, st.Size(), entry.Size)

				err = filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			}
		}

		logger.Debugf("uploading a local file %s to %s", sourcePath, targetFilePath)
		err := filesystem.UploadFileParallel(sourcePath, targetPath, "", 0, true)
		if err != nil {
			return err
		}
	} else {
		// dir
		logger.Debugf("uploading a collection %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}

		// make target dir if not exists
		if !filesystem.ExistsDir(targetPath) {
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return err
			}
		}

		for _, entryInDir := range entries {
			targetEntryPath := filepath.Join(targetPath, entryInDir.Name())
			err = syncPutOne(filesystem, filepath.Join(sourcePath, entryInDir.Name()), targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncCopyOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "syncCopyOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := filesystem.Stat(sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		targetFilePath := commons.EnsureTargetIRODSFilePath(filesystem, sourcePath, targetPath)

		if filesystem.Exists(targetFilePath) {
			// already exists!
			if filesystem.ExistsDir(targetFilePath) {
				// dir
				logger.Debugf("path %s is a collection, deleting", targetFilePath)
				err = filesystem.RemoveDir(targetFilePath, true, true)
				if err != nil {
					return err
				}
			} else {
				// file
				entry, err := filesystem.Stat(targetFilePath)
				if err != nil {
					return err
				}

				if entry.CheckSum == sourceEntry.CheckSum && entry.Size == sourceEntry.Size {
					// match
					logger.Debugf("data object %s is up-to-date", targetFilePath)
					return nil
				}

				// delete first
				logger.Debugf("data object %s is has a different hash code, deleting", targetFilePath)
				logger.Debugf(" hash %s vs. %s, size %d vs. %d", sourceEntry.CheckSum, entry.CheckSum, sourceEntry.Size, entry.Size)
				err = filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return err
				}
			}
		}

		logger.Debugf("copying a data object %s to %s", sourcePath, targetFilePath)
		err = filesystem.CopyFileToFile(sourcePath, targetFilePath)
		if err != nil {
			return err
		}
	} else {
		// dir
		logger.Debugf("copying a collection %s to %s", sourcePath, targetPath)

		entries, err := filesystem.List(sourceEntry.Path)
		if err != nil {
			return err
		}

		// make target dir if not exists
		if !filesystem.ExistsDir(targetPath) {
			err = filesystem.MakeDir(targetPath, true)
			if err != nil {
				return err
			}
		}

		for _, entryInDir := range entries {
			targetEntryPath := filepath.Join(targetPath, entryInDir.Name)
			err = syncCopyOne(filesystem, entryInDir.Path, targetEntryPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func hashLocalFileMD5(sourcePath string) (string, error) {
	hashAlg := md5.New()
	return hashLocalFile(sourcePath, hashAlg)
}

func hashLocalFile(sourcePath string, hashAlg hash.Hash) (string, error) {
	f, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}

	defer f.Close()

	_, err = io.Copy(hashAlg, f)
	if err != nil {
		return "", err
	}

	sumBytes := hashAlg.Sum(nil)
	sumString := hex.EncodeToString(sumBytes)

	return sumString, nil
}
