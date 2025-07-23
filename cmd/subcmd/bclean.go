package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/bundle"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var bcleanCmd = &cobra.Command{
	Use:     "bclean",
	Aliases: []string{"bundle_clean"},
	Short:   "Clear local bundle creation and iRODS bundle staging directories",
	Long:    `This command removes the bundle files created during 'bput' or 'sync' operations for uploading data to an iRODS collection. It helps free up space by cleaning both local bundle creation directories and temporary staging areas in iRODS.`,
	RunE:    processBcleanCommand,
}

func AddBcleanCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(bcleanCmd, false)

	flag.SetBundleTransferFlags(bcleanCmd, false, true)

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

	commonFlagValues         *flag.CommonFlagValues
	bundleTransferFlagValues *flag.BundleTransferFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetPaths []string
}

func NewBcleanCommand(command *cobra.Command, args []string) (*BcleanCommand, error) {
	bclean := &BcleanCommand{
		command: command,

		commonFlagValues:         flag.GetCommonFlagValues(command),
		bundleTransferFlagValues: flag.GetBundleTransferFlagValues(),
	}

	// path
	bclean.targetPaths = args

	return bclean, nil
}

func (bclean *BcleanCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(bclean.command)
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
	bclean.account = config.GetSessionConfig().ToIRODSAccount()
	bclean.filesystem, err = irods.GetIRODSFSClient(bclean.account, false, false)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer bclean.filesystem.Release()

	if bclean.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(bclean.filesystem, bclean.commonFlagValues.Timeout)
	}

	// run
	for _, targetPath := range bclean.targetPaths {
		bclean.cleanOne(targetPath)
	}

	return nil
}

func (bclean *BcleanCommand) cleanOne(targetPath string) {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := bclean.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	// bundle manager
	stagingPath := bclean.bundleTransferFlagValues.IRODSTempPath
	if len(stagingPath) == 0 {
		stagingPath = bundle.GetStagingDirInTargetPath(targetPath)
	}

	bundleManager := bundle.NewBundleManager(bclean.bundleTransferFlagValues.MinFileNumInBundle, bclean.bundleTransferFlagValues.MaxFileNumInBundle, bclean.bundleTransferFlagValues.MaxBundleFileSize, bclean.bundleTransferFlagValues.LocalTempPath, stagingPath)

	bundleManager.ClearLocalBundles()
	bundleManager.ClearIRODSBundles(bclean.filesystem, true)
}
