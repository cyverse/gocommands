package subcmd

import (
	"os"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
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
	commons.SetCommonFlags(bcleanCmd)

	bcleanCmd.Flags().BoolP("force", "f", false, "Put forcefully")
	bcleanCmd.Flags().String("local_temp", os.TempDir(), "Specify local temp directory path to create bundle files")
	bcleanCmd.Flags().String("irods_temp", "", "Specify iRODS temp directory path to upload bundle files to")

	rootCmd.AddCommand(bcleanCmd)
}

func processBcleanCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBcleanCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
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

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	localTempDirPath := os.TempDir()
	localTempPathFlag := command.Flags().Lookup("local_temp")
	if localTempPathFlag != nil {
		localTempDirPath = localTempPathFlag.Value.String()
	}

	irodsTempDirPath := ""
	irodsTempPathFlag := command.Flags().Lookup("irods_temp")
	if irodsTempPathFlag != nil {
		tempDirPath := irodsTempPathFlag.Value.String()
		if len(tempDirPath) > 0 {
			irodsTempDirPath = tempDirPath
		}
	}

	// clear local
	commons.CleanUpOldLocalBundles(localTempDirPath, force)

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	trashHome := commons.GetTrashHomeDir()
	logger.Debugf("clearing trash dir %s", trashHome)
	commons.CleanUpOldIRODSBundles(filesystem, trashHome, false, force)

	if len(irodsTempDirPath) > 0 {
		logger.Debugf("clearing irods temp dir %s", irodsTempDirPath)
		commons.CleanUpOldIRODSBundles(filesystem, irodsTempDirPath, false, force)
	}

	for _, targetPath := range args {
		bcleanOne(filesystem, targetPath, force)
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
