package subcmd

import (
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bunCmd = &cobra.Command{
	Use:     "bun [data-object1] [data-object2] ... [target collection]",
	Aliases: []string{"bundle", "ibun"},
	Short:   "Extract iRODS data-objects in a structured file format to target collection",
	Long:    `This extracts iRODS data-objects in a structured file format (e.g., zip and tar) to the given target collection.`,
	RunE:    processBunCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddBunCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bunCmd)

	flag.SetForceFlags(bunCmd, false)
	flag.SetBundleFlags(bunCmd)

	rootCmd.AddCommand(bunCmd)
}

func processBunCommand(command *cobra.Command, args []string) error {
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	forceFlagValues := flag.GetForceFlagValues()
	bundleFlagValues := flag.GetBundleFlagValues()

	if !bundleFlagValues.Extract {
		return xerrors.Errorf("support only extract mode")
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	targetPath := args[len(args)-1]
	for _, sourcePath := range args[:len(args)-1] {
		if bundleFlagValues.Extract {
			err = extractOne(filesystem, sourcePath, targetPath, bundleFlagValues.DataType, forceFlagValues.Force, bundleFlagValues.BulkRegistration)
			if err != nil {
				return xerrors.Errorf("failed to perform bun %s to %s: %w", sourcePath, targetPath, err)
			}
		}
	}

	return nil
}

func getDataType(irodsPath string, dataType string) (irodsclient_types.DataType, error) {
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
		return "", xerrors.Errorf("unknown format %s", dataType)
	}

	// auto
	ext := commons.GetFileExtension(irodsPath)
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

func extractOne(filesystem *irodsclient_fs.FileSystem, sourcePath string, targetPath string, dataType string, force bool, bulkReg bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "extractOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := commons.StatIRODSPath(filesystem, sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	targetEntry, err := commons.StatIRODSPath(filesystem, targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
		}
	} else {
		if targetEntry.Type == irodsclient_fs.FileEntry {
			return xerrors.Errorf("%s is not a collection", targetPath)
		}
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("extracting a data object %s to %s", sourcePath, targetPath)

		dt, err := getDataType(sourcePath, dataType)
		if err != nil {
			return xerrors.Errorf("failed to get type %s: %w", sourcePath, err)
		}

		err = filesystem.ExtractStructFile(sourcePath, targetPath, "", dt, force, bulkReg)
		if err != nil {
			return xerrors.Errorf("failed to extract file %s to %s: %w", sourcePath, targetPath, err)
		}
	} else {
		return xerrors.Errorf("source %s must be a data object", sourcePath)
	}
	return nil
}
