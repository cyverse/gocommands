package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bcleanCmd = &cobra.Command{
	Use:     "bclean [collection]",
	Aliases: []string{"bundle_clean"},
	Short:   "Clean bundle staging directories",
	Long:    `This cleans bundle files created by 'bput' or 'sync' for uploading data to the given iRODS collection.`,
	RunE:    processBcleanCommand,
}

func AddBcleanCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bcleanCmd, false)

	// attach bundle temp flags
	flag.SetBundleTempFlags(bcleanCmd)
	flag.SetForceFlags(bcleanCmd, false)

	rootCmd.AddCommand(bcleanCmd)
}

func processBcleanCommand(command *cobra.Command, args []string) error {
	bclean, err := NewBcleanCommand(command, args)
	if err != nil {
		return err
	}

	return bclean.Process()
}

type BcleanCommand struct {
	command *cobra.Command

	forceFlagValues      *flag.ForceFlagValues
	bundleTempFlagValues *flag.BundleTempFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewBcleanCommand(command *cobra.Command, args []string) (*BcleanCommand, error) {
	bclean := &BcleanCommand{
		command: command,

		forceFlagValues:      flag.GetForceFlagValues(),
		bundleTempFlagValues: flag.GetBundleTempFlagValues(),
	}

	// path
	bclean.targetPaths = args

	return bclean, nil
}

func (bclean *BcleanCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BcleanCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(bclean.command)
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

	// Create a file system
	bclean.account = commons.GetAccount()
	bclean.filesystem, err = commons.GetIRODSFSClient(bclean.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bclean.filesystem.Release()

	// run
	// clear local
	commons.CleanUpOldLocalBundles(bclean.bundleTempFlagValues.LocalTempPath, bclean.forceFlagValues.Force)

	// clear remote
	if len(bclean.bundleTempFlagValues.IRODSTempPath) > 0 {
		logger.Debugf("clearing an irods temp directory %q", bclean.bundleTempFlagValues.IRODSTempPath)

		commons.CleanUpOldIRODSBundles(bclean.filesystem, bclean.bundleTempFlagValues.IRODSTempPath, true, bclean.forceFlagValues.Force)
	} else {
		userHome := commons.GetHomeDir()
		homeStagingDir := commons.GetDefaultStagingDir(userHome)
		commons.CleanUpOldIRODSBundles(bclean.filesystem, homeStagingDir, true, bclean.forceFlagValues.Force)
	}

	for _, targetPath := range bclean.targetPaths {
		bclean.cleanOne(targetPath)
	}

	return nil
}

func (bclean *BcleanCommand) cleanOne(targetPath string) {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "BcleanCommand",
		"function": "cleanOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	if commons.IsStagingDirInTargetPath(targetPath) {
		// target is staging dir
		logger.Debugf("clearing an irods target directory %q", targetPath)
		commons.CleanUpOldIRODSBundles(bclean.filesystem, targetPath, true, bclean.forceFlagValues.Force)
		return
	}

	stagingDirPath := commons.GetDefaultStagingDirInTargetPath(targetPath)
	logger.Debugf("clearing an irods target directory %q", stagingDirPath)

	commons.CleanUpOldIRODSBundles(bclean.filesystem, stagingDirPath, true, bclean.forceFlagValues.Force)
}
