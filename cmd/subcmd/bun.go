package subcmd

import (
	"path"
	"strings"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	commons_path "github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/types"
	"github.com/cyverse/gocommands/commons/wildcard"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bunCmd = &cobra.Command{
	Use:     "bun <data-object>... <target-collection>",
	Aliases: []string{"bundle", "ibun"},
	Short:   "Extract iRODS data objects to a target collection",
	Long:    `This command extracts iRODS data objects (e.g., zip, tar) to the specified target collection.`,

	RunE: processBunCommand,
	Args: cobra.MinimumNArgs(2),
}

func AddBunCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bunCmd, false)

	flag.SetForceFlags(bunCmd, false)
	flag.SetBundleFlags(bunCmd)
	flag.SetWildcardSearchFlags(bunCmd)

	rootCmd.AddCommand(bunCmd)
}

func processBunCommand(command *cobra.Command, args []string) error {
	bun, err := NewBunCommand(command, args)
	if err != nil {
		return err
	}

	return bun.Process()
}

type BunCommand struct {
	command *cobra.Command

	commonFlagValues         *flag.CommonFlagValues
	forceFlagValues          *flag.ForceFlagValues
	bundleFlagValues         *flag.BundleFlagValues
	wildcardSearchFlagValues *flag.WildcardSearchFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
	targetPath  string
}

func NewBunCommand(command *cobra.Command, args []string) (*BunCommand, error) {
	bun := &BunCommand{
		command: command,

		commonFlagValues:         flag.GetCommonFlagValues(command),
		forceFlagValues:          flag.GetForceFlagValues(),
		bundleFlagValues:         flag.GetBundleFlagValues(),
		wildcardSearchFlagValues: flag.GetWildcardSearchFlagValues(),
	}

	// path
	bun.targetPath = args[len(args)-1]
	bun.sourcePaths = args[:len(args)-1]

	if !bun.bundleFlagValues.Extract {
		return nil, xerrors.Errorf("support only extract mode")
	}

	return bun, nil
}

func (bun *BunCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(bun.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	bun.account = config.GetSessionConfig().ToIRODSAccount()
	bun.filesystem, err = irods.GetIRODSFSClient(bun.account, false, true)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bun.filesystem.Release()

	if bun.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(bun.filesystem, bun.commonFlagValues.Timeout)
	}

	// Expand wildcards
	if bun.wildcardSearchFlagValues.WildcardSearch {
		bun.sourcePaths, err = wildcard.ExpandWildcards(bun.filesystem, bun.account, bun.sourcePaths, false, true)
		if err != nil {
			return xerrors.Errorf("failed to expand wildcards: %w", err)
		}
	}

	// run
	for _, sourcePath := range bun.sourcePaths {
		if bun.bundleFlagValues.Extract {
			err = bun.extractOne(sourcePath, bun.targetPath)
			if err != nil {
				return xerrors.Errorf("failed to extract bundle file %q to %q: %w", sourcePath, bun.targetPath, err)
			}
		}
	}

	return nil
}

func (bun *BunCommand) getDataType(irodsPath string, dataType string) (irodsclient_types.DataType, error) {
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
		return "", xerrors.Errorf("unknown format %q", dataType)
	}

	// auto
	ext := path.Ext(irodsPath)
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

func (bun *BunCommand) extractOne(sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BunCommand",
		"function": "extractOne",
	})

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := bun.account.ClientZone
	sourcePath = commons_path.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons_path.MakeIRODSPath(cwd, home, zone, targetPath)

	sourceEntry, err := bun.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	targetEntry, err := bun.filesystem.Stat(targetPath)
	if err != nil {
		if !irodsclient_types.IsFileNotFoundError(err) {
			return xerrors.Errorf("failed to stat %q: %w", targetPath, err)
		}
	} else {
		if !targetEntry.IsDir() {
			return types.NewNotDirError(targetPath)
		}
	}

	if sourceEntry.IsDir() {
		return xerrors.Errorf("source %q must be a data object", sourcePath)
	}

	// file
	logger.Debugf("extracting a data object %q to %q", sourcePath, targetPath)

	dt, err := bun.getDataType(sourcePath, bun.bundleFlagValues.DataType)
	if err != nil {
		return xerrors.Errorf("failed to get type %q: %w", sourcePath, err)
	}

	err = bun.filesystem.ExtractStructFile(sourcePath, targetPath, "", dt, bun.forceFlagValues.Force, bun.bundleFlagValues.BulkRegistration)
	if err != nil {
		return xerrors.Errorf("failed to extract file %q to %q: %w", sourcePath, targetPath, err)
	}

	return nil
}
