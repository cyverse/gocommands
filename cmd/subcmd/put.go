package subcmd

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/cyverse/go-irodsclient/fs"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var putCmd = &cobra.Command{
	Use:   "put [local file1] [local file2] [local dir1] ... [collection]",
	Short: "Upload files or directories",
	Long:  `This uploads files or directories to the given iRODS collection.`,
	RunE:  processPutCommand,
}

func AddPutCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(putCmd)

	putCmd.Flags().BoolP("force", "f", false, "Put forcefully")
	putCmd.Flags().Bool("single_threaded", false, "Transfer a file using a single thread")
	putCmd.Flags().Int("upload_thread_num", commons.MaxParallelJobThreadNumDefault, "Specify the number of upload threads")
	putCmd.Flags().String("tcp_buffer_size", commons.TcpBufferSizeStringDefault, "Specify TCP socket buffer size")
	putCmd.Flags().Bool("progress", false, "Display progress bar")
	putCmd.Flags().Bool("diff", false, "Put files having different content")
	putCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")
	putCmd.Flags().Bool("no_replication", false, "Disable replication")
	putCmd.Flags().Int("retry", 1, "Retry if fails")
	putCmd.Flags().Int("retry_interval", 60, "Retry interval in seconds")

	rootCmd.AddCommand(putCmd)
}

func processPutCommand(command *cobra.Command, args []string) error {
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

	singleThreaded := false
	singleThreadedFlag := command.Flags().Lookup("single_threaded")
	if singleThreadedFlag != nil {
		singleThreaded, err = strconv.ParseBool(singleThreadedFlag.Value.String())
		if err != nil {
			singleThreaded = false
		}
	}

	uploadThreadNum := commons.MaxParallelJobThreadNumDefault
	uploadThreadNumFlag := command.Flags().Lookup("upload_thread_num")
	if uploadThreadNumFlag != nil {
		n, err := strconv.ParseInt(uploadThreadNumFlag.Value.String(), 10, 32)
		if err == nil {
			uploadThreadNum = int(n)
		}
	}

	maxConnectionNum := uploadThreadNum + 2 // 2 for metadata op

	tcpBufferSize := commons.TcpBufferSizeDefault
	tcpBufferSizeFlag := command.Flags().Lookup("tcp_buffer_size")
	if tcpBufferSizeFlag != nil {
		n, err := commons.ParseSize(tcpBufferSizeFlag.Value.String())
		if err == nil {
			tcpBufferSize = int(n)
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

	diff := false
	diffFlag := command.Flags().Lookup("diff")
	if diffFlag != nil {
		diff, err = strconv.ParseBool(diffFlag.Value.String())
		if err != nil {
			diff = false
		}
	}

	noHash := false
	noHashFlag := command.Flags().Lookup("no_hash")
	if noHashFlag != nil {
		noHash, err = strconv.ParseBool(noHashFlag.Value.String())
		if err != nil {
			noHash = false
		}
	}

	noReplication := false
	noReplicationFlag := command.Flags().Lookup("no_replication")
	if noReplicationFlag != nil {
		noReplication, err = strconv.ParseBool(noReplicationFlag.Value.String())
		if err != nil {
			noReplication = false
		}
	}

	replication := !noReplication

	retryChild := false
	retryChildFlag := command.Flags().Lookup("retry_child")
	if retryChildFlag != nil {
		retryChildValue, err := strconv.ParseBool(retryChildFlag.Value.String())
		if err != nil {
			retryChildValue = false
		}

		retryChild = retryChildValue
	}

	retry := int64(1)
	retryFlag := command.Flags().Lookup("retry")
	if retryFlag != nil {
		retry, err = strconv.ParseInt(retryFlag.Value.String(), 10, 32)
		if err != nil {
			retry = 1
		}
	}

	retryInterval := int64(60)
	retryIntervalFlag := command.Flags().Lookup("retry_interval")
	if retryIntervalFlag != nil {
		retryInterval, err = strconv.ParseInt(retryIntervalFlag.Value.String(), 10, 32)
		if err != nil {
			retryInterval = 60
		}
	}

	if retry > 1 && !retryChild {
		err = commons.RunWithRetry(int(retry), int(retryInterval))
		if err != nil {
			return xerrors.Errorf("failed to run with retry %d: %w", retry, err)
		}
		return nil
	}

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClientAdvanced(account, maxConnectionNum, tcpBufferSize)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	if len(args) == 0 {
		return xerrors.Errorf("not enough input arguments")
	}

	targetPath := "./"
	sourcePaths := args[:]

	if len(args) >= 2 {
		targetPath = args[len(args)-1]
		sourcePaths = args[:len(args)-1]
	}

	parallelJobManager := commons.NewParallelJobManager(filesystem, uploadThreadNum, progress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		err = putOne(parallelJobManager, sourcePath, targetPath, force, singleThreaded, replication, diff, noHash)
		if err != nil {
			return xerrors.Errorf("failed to perform put %s to %s: %w", sourcePath, targetPath, err)
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		return xerrors.Errorf("failed to perform parallel jobs: %w", err)
	}

	return nil
}

func putOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, force bool, singleThreaded bool, replicate bool, diff bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "putOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeLocalPath(sourcePath)
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceStat, err := os.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", sourcePath, err)
	}

	if !sourceStat.IsDir() {
		// file
		targetFilePath := commons.MakeTargetIRODSFilePath(filesystem, sourcePath, targetPath)
		exist := commons.ExistsIRODSFile(filesystem, targetFilePath)

		putTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackPut := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceStat.Size(), false)

			logger.Debugf("uploading a file %s to %s", sourcePath, targetFilePath)
			if singleThreaded {
				err = fs.UploadFile(sourcePath, targetFilePath, "", replicate, callbackPut)
			} else {
				err = fs.UploadFileParallel(sourcePath, targetFilePath, "", 0, replicate, callbackPut)
			}

			if err != nil {
				job.Progress(-1, sourceStat.Size(), true)
				return xerrors.Errorf("failed to upload %s to %s: %w", sourcePath, targetFilePath, err)
			}

			logger.Debugf("uploaded a file %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceStat.Size(), sourceStat.Size(), false)
			return nil
		}

		if exist {
			targetEntry, err := commons.StatIRODSPath(filesystem, targetFilePath)
			if err != nil {
				return xerrors.Errorf("failed to stat %s: %w", targetFilePath, err)
			}

			if diff {
				if noHash {
					if targetEntry.Size == sourceStat.Size() {
						fmt.Printf("skip uploading a file %s. The file already exists!\n", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size == sourceStat.Size() {
						if len(targetEntry.CheckSum) > 0 {
							// compare hash
							md5hash, err := commons.HashLocalFileMD5(sourcePath)
							if err != nil {
								return xerrors.Errorf("failed to get hash for %s: %w", sourcePath, err)
							}

							if md5hash == targetEntry.CheckSum {
								fmt.Printf("skip uploading a file %s. The file with the same hash already exists!\n", targetFilePath)
								return nil
							}
						}
					}
				}

				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else if force {
				logger.Debugf("deleting an existing data object %s", targetFilePath)
				err := filesystem.RemoveFile(targetFilePath, true)
				if err != nil {
					return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing data object %s", targetFilePath)
					err := filesystem.RemoveFile(targetFilePath, true)
					if err != nil {
						return xerrors.Errorf("failed to remove %s: %w", targetFilePath, err)
					}
				} else {
					fmt.Printf("skip uploading a file %s. The data object already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		threadsRequired := computeThreadsRequiredForPut(filesystem, singleThreaded, sourceStat.Size())
		parallelJobManager.Schedule(sourcePath, putTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a local file upload %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("uploading a local directory %s to %s", sourcePath, targetPath)

		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to read dir %s: %w", sourcePath, err)
		}

		// make target dir
		targetDir := path.Join(targetPath, filepath.Base(sourcePath))
		err = filesystem.MakeDir(targetDir, true)
		if err != nil {
			return xerrors.Errorf("failed to make dir %s: %w", targetDir, err)
		}

		for _, entryInDir := range entries {
			newSourcePath := filepath.Join(sourcePath, entryInDir.Name())
			err = putOne(parallelJobManager, newSourcePath, targetDir, force, singleThreaded, replicate, diff, noHash)
			if err != nil {
				return xerrors.Errorf("failed to perform put %s to %s: %w", newSourcePath, targetDir, err)
			}
		}
	}
	return nil
}

func computeThreadsRequiredForPut(fs *fs.FileSystem, singleThreaded bool, size int64) int {
	if singleThreaded {
		return 1
	}

	if fs.SupportParallelUpload() {
		return irodsclient_util.GetNumTasksForParallelTransfer(size)
	}

	return 1
}
