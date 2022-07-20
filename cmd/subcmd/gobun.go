package subcmd

import (
	"fmt"
	"strconv"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var bunCmd = &cobra.Command{
	Use:   "bun [data-object1] [data-object2] ... [target collection]",
	Short: "Extract iRODS data-objects in a structured file format to target collection",
	Long:  `This extracts iRODS data-objects in a structured file format (e.g., zip and tar) to the given target collection.`,
	RunE:  processBunCommand,
}

func AddBunCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(bunCmd)
	bunCmd.Flags().BoolP("force", "f", false, "Extract forcefully")
	bunCmd.Flags().BoolP("extract", "x", false, "Extract data-objects")
	bunCmd.Flags().StringP("data_type", "D", "", "Set data type (tar, zip ...)")

	rootCmd.AddCommand(bunCmd)
}

func processBunCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBunCommand",
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	extract := false
	extractFlag := command.Flags().Lookup("extract")
	if extractFlag != nil {
		extract, err = strconv.ParseBool(extractFlag.Value.String())
		if err != nil {
			extract = false
		}
	}

	// TODO: currently only extract is supported
	extract = true

	dataType := "" // auto
	dataTypeFlag := command.Flags().Lookup("data_type")
	if dataTypeFlag != nil {
		dataType = dataTypeFlag.Value.String()
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
			if extract {
				err = extractOne(filesystem, sourcePath, targetPath, dataType, force)
				if err != nil {
					logger.Error(err)
					return err
				}
			}
		}
	} else {
		return fmt.Errorf("arguments given are not sufficent")
	}
	return nil
}

func getDataType(path string, dataType string) (irodsclient_types.DataType, error) {
	switch strings.ToLower(dataType) {
	case "tar", "t", "tar file":
		return irodsclient_types.TAR_FILE_DT, nil
	case "g", "gzip", "gziptar":
		return irodsclient_types.GZIP_TAR_DT, nil
	case "b", "bzip2", "bzip2tar":
		return irodsclient_types.BZIP2_TAR_DT, nil
	case "z", "zip", "zipfile":
		return irodsclient_types.ZIP_FILE_DT, nil
	case "":
		// auto
	default:
		return "", fmt.Errorf("unknown format %s", dataType)
	}

	// auto
	ext := commons.GetFileExtension(path)
	switch strings.ToLower(ext) {
	case ".tar":
		return irodsclient_types.TAR_FILE_DT, nil
	case ".tar.gz":
		return irodsclient_types.GZIP_TAR_DT, nil
	case ".tar.bz2":
		return irodsclient_types.BZIP2_TAR_DT, nil
	case ".zip":
		return irodsclient_types.ZIP_FILE_DT, nil
	default:
		return irodsclient_types.TAR_FILE_DT, nil
	}
}

func extractOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, dataType string, force bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "extractOne",
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

	targetEntry, err := filesystem.Stat(targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return err
		}
	} else {
		if targetEntry.Type == irodsclient_fs.FileEntry {
			return fmt.Errorf("%s is not a collection", targetPath)
		}
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("extracting a data object %s to %s", sourcePath, targetPath)

		dt, err := getDataType(sourcePath, dataType)
		if err != nil {
			return err
		}

		err = filesystem.ExtractStructFile(sourcePath, targetPath, "", dt, force)
		if err != nil {
			return err
		}
	} else {
		return fmt.Errorf("source must be a data object")
	}
	return nil
}
