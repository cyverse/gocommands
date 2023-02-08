package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cyverse/gocommands/commons"
	"github.com/rs/xid"
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
	bputCmd.Flags().String("job_id", "", "Specify Job ID")
	bputCmd.Flags().Bool("continue", false, "Continue from last failure point")

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

	jobID := xid.New().String()
	jobIDFlag := command.Flags().Lookup("job_id")
	if jobIDFlag != nil {
		jobID = jobIDFlag.Value.String()
	}

	continueFromFailure := false
	continueFlag := command.Flags().Lookup("continue")
	if continueFlag != nil {
		continueFromFailure, err = strconv.ParseBool(continueFlag.Value.String())
		if err != nil {
			continueFromFailure = false
		}
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

	if len(args) == 0 {
		err := fmt.Errorf("not enough input arguments")
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	targetPath := "./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)
	irodsTempDirPath := commons.MakeIRODSPath(cwd, home, zone, "./")

	if len(jobID) == 0 {
		jobID = xid.New().String()
	}

	jobFile := commons.GetDefaultJobLogPath(jobID)

	var jobLog *commons.JobLog
	if continueFromFailure {
		jobLogExisting, err := commons.NewJobLogFromLog(jobFile)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}

		jobLog = jobLogExisting
	} else {
		jobLog = commons.NewJobLog(jobID, jobFile, sourcePaths, targetPath)
		err = jobLog.MakeJobLogDir()
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	bundleTransferManager := commons.NewBundleTransferManager(jobLog, filesystem, targetPath, maxFileNum, maxFileSize, localTempDirPath, irodsTempDirPath, force, progress)
	bundleTransferManager.Start()

	bundleRootPath, err := commons.GetCommonRootLocalDirPath(sourcePaths)
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	bundleTransferManager.SetBundleRootPath(bundleRootPath)

	err = jobLog.WriteHeader()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	jobLog.MonitorCtrlC()

	for _, sourcePath := range sourcePaths {
		err = bputOne(bundleTransferManager, sourcePath, targetPath)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			jobLog.PrintJobID()
			return nil
		}
	}

	bundleTransferManager.DoneScheduling()
	err = bundleTransferManager.Wait()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		jobLog.PrintJobID()
		return nil
	}

	return nil
}

func bputOne(bundleManager *commons.BundleTransferManager, sourcePath string, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "bputOne",
	})

	sourcePath = commons.MakeLocalPath(sourcePath)

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !sourceStat.IsDir() {
		logger.Debugf("scheduled a local file bundle-upload %s", sourcePath)
		bundleManager.Schedule(sourcePath, sourceStat.Size(), sourceStat.ModTime().Local())
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
			bundleManager.Schedule(path, info.Size(), info.ModTime())

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
