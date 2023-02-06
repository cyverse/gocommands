package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var bputCmd = &cobra.Command{
	Use:   "bput [local file1] [local file2] [local dir1] ... [collection]",
	Short: "Bundle-upload files or directories",
	Long:  `This uploads files or directories to the given iRODS collection. The files or directories are bundled with TAR to maximize data transfer bandwidth, then extracted in the iRODS.`,
	RunE:  processBputCommand,
}

func AddBputCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(bputCmd)

	bputCmd.Flags().BoolP("force", "f", false, "Put forcefully (overwrite)")
	bputCmd.Flags().Int("max_file_num", commons.MaxBundleFileNumDefault, "Specify max file number in a bundle file")
	bputCmd.Flags().Int64("max_file_size", commons.MaxBundleFileSizeDefault, "Specify max file size of a bundle file")
	bputCmd.Flags().Bool("progress", false, "Display progress bar")
	bputCmd.Flags().String("local_temp", os.TempDir(), "Specify a local temp directory path to create bundle files")

	rootCmd.AddCommand(bputCmd)
}

func processBputCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processBputCommand",
	})

	cont, err := commons.ProcessCommonFlags(command)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = commons.InputMissingFields()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	force := false
	forceFlag := command.Flags().Lookup("force")
	if forceFlag != nil {
		force, err = strconv.ParseBool(forceFlag.Value.String())
		if err != nil {
			force = false
		}
	}

	maxFileNum := commons.MaxBundleFileNumDefault
	maxFileNumFlag := command.Flags().Lookup("max_file_num")
	if maxFileNumFlag != nil {
		n, err := strconv.ParseInt(maxFileNumFlag.Value.String(), 10, 32)
		if err == nil {
			maxFileNum = int(n)
		}
	}

	maxFileSize := commons.MaxBundleFileSizeDefault
	maxFileSizeFlag := command.Flags().Lookup("max_file_size")
	if maxFileSizeFlag != nil {
		n, err := strconv.ParseInt(maxFileSizeFlag.Value.String(), 10, 64)
		if err == nil {
			maxFileSize = n
		}
	}

	progress := false
	progressFlag := command.Flags().Lookup("progress")
	if progressFlag != nil {
		progress, err = strconv.ParseBool(progressFlag.Value.String())
		if err != nil {
			progress = false
		}
	}

	localTempDirPath := os.TempDir()
	localTempPathFlag := command.Flags().Lookup("local_temp")
	if localTempPathFlag != nil {
		localTempDirPath = localTempPathFlag.Value.String()
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	defer filesystem.Release()

	targetPath := "./"
	if len(args) >= 2 {
		targetPath = args[len(args)-1]
	}

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)
	irodsTempDirPath := commons.MakeIRODSPath(cwd, home, zone, "./")

	bundleTransferManager := commons.NewBundleTransferManager(filesystem, targetPath, maxFileNum, maxFileSize, localTempDirPath, irodsTempDirPath, force, progress)
	bundleTransferManager.Start()

	if len(args) == 1 {
		// upload to current collection
		bundleRootPath, err := commons.GetCommonRootLocalDirPath(args)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}

		err = bputOne(bundleTransferManager, args[0], bundleRootPath)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	} else if len(args) >= 2 {
		targetPath = args[len(args)-1]

		bundleRootPath, err := commons.GetCommonRootLocalDirPath(args[:len(args)-1])
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}

		for _, sourcePath := range args[:len(args)-1] {
			err = bputOne(bundleTransferManager, sourcePath, bundleRootPath)
			if err != nil {
				logger.Error(err)
				fmt.Fprintln(os.Stderr, err.Error())
				return nil
			}
		}
	} else {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	bundleTransferManager.DoneScheduling()
	err = bundleTransferManager.Wait()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func bputOne(bundleManager *commons.BundleTransferManager, sourcePath string, bundleRootPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "bputOne",
	})

	bundleManager.SetBundleRootPath(bundleRootPath)

	sourcePath = commons.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !sourceStat.IsDir() {
		logger.Debugf("scheduled a local file bundle-upload %s", sourcePath)
		bundleManager.Schedule(sourcePath, sourceStat.Size())
	} else {
		// dir
		logger.Debugf("bundle-uploading a local directory %s", sourcePath)

		walkFunc := func(path string, entry os.DirEntry, err2 error) error {
			if err2 != nil {
				return err2
			}

			if entry.IsDir() {
				return nil
			}

			info, err := entry.Info()
			if err != nil {
				return err
			}

			logger.Debugf("> scheduled a local file bundle-upload %s", path)
			bundleManager.Schedule(path, info.Size())

			if err != nil {
				return err
			}
			return nil
		}

		err := filepath.WalkDir(sourcePath, walkFunc)
		if err != nil {
			return err
		}
	}
	return nil
}
