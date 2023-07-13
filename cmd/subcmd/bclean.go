package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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
	flag.SetCommonFlags(bcleanCmd)

	// attach bundle temp flags
	flag.SetBundleTempFlags(bcleanCmd)
	flag.SetForceFlags(bcleanCmd, false)

	rootCmd.AddCommand(bcleanCmd)
}

func processBcleanCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBcleanCommand",
	})

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
	bundleTempFlagValues := flag.GetBundleTempFlagValues()

	// clear local
	commons.CleanUpOldLocalBundles(bundleTempFlagValues.LocalTempPath, forceFlagValues.Force)

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(bundleTempFlagValues.IRODSTempPath) > 0 {
		logger.Debugf("clearing irods temp dir %s", bundleTempFlagValues.IRODSTempPath)
		commons.CleanUpOldIRODSBundles(filesystem, bundleTempFlagValues.IRODSTempPath, true, forceFlagValues.Force)
	} else {
		userHome := commons.GetHomeDir()
		homeStagingDir := commons.GetDefaultStagingDir(userHome)
		commons.CleanUpOldIRODSBundles(filesystem, homeStagingDir, true, forceFlagValues.Force)
	}

	for _, targetPath := range args {
		bcleanOne(filesystem, targetPath, forceFlagValues.Force)
	}

	return nil
}

func bcleanOne(fs *irodsclient_fs.FileSystem, targetPath string, force bool) {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "bcleanOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	if commons.IsStagingDirInTargetPath(targetPath) {
		// target is staging dir
		logger.Debugf("clearing irods target dir %s", targetPath)
		commons.CleanUpOldIRODSBundles(fs, targetPath, true, force)
		return
	}

	stagingDirPath := commons.GetDefaultStagingDirInTargetPath(targetPath)
	logger.Debugf("clearing irods target dir %s", stagingDirPath)

	commons.CleanUpOldIRODSBundles(fs, stagingDirPath, true, force)
}
