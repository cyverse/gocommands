package subcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	"github.com/jedib0t/go-pretty/v6/progress"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var getCmd = &cobra.Command{
	Use:   "get [data-object1] [data-object2] [collection1] ... [local dir]",
	Short: "Download iRODS data-objects or collections",
	Long:  `This downloads iRODS data-objects or collections to the given local path.`,
	RunE:  processGetCommand,
}

func AddGetCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(getCmd)

	getCmd.Flags().BoolP("force", "f", false, "Get forcefully")
	getCmd.Flags().Bool("progress", false, "Display progress bar")
	getCmd.Flags().Bool("diff", false, "Get files having different content")
	getCmd.Flags().Bool("no_hash", false, "Compare files without using md5 hash")

	rootCmd.AddCommand(getCmd)
}

func processGetCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "processGetCommand",
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

	parallelJobManager := commons.NewParallelJobManager(filesystem, commons.MaxThreadNumDefault, progress)
	parallelJobManager.Start()

	for _, sourcePath := range sourcePaths {
		err = getOne(parallelJobManager, sourcePath, targetPath, force, diff, noHash)
		if err != nil {
			logger.Error(err)
			fmt.Fprintln(os.Stderr, err.Error())
			return nil
		}
	}

	parallelJobManager.DoneScheduling()
	err = parallelJobManager.Wait()
	if err != nil {
		logger.Error(err)
		fmt.Fprintln(os.Stderr, err.Error())
		return nil
	}

	return nil
}

func getOne(parallelJobManager *commons.ParallelJobManager, sourcePath string, targetPath string, force bool, diff bool, noHash bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "getOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)
	targetPath = commons.MakeLocalPath(targetPath)

	filesystem := parallelJobManager.GetFilesystem()

	sourceEntry, err := commons.StatIRODSPath(filesystem, sourcePath)
	if err != nil {
		return err
	}

	if sourceEntry.Type == irodsclient_fs.FileEntry {
		targetFilePath := commons.MakeTargetLocalFilePath(sourcePath, targetPath)

		exist := false
		targetEntry, err := os.Stat(targetFilePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
		} else {
			exist = true
		}

		getTask := func(job *commons.ParallelJob) error {
			manager := job.GetManager()
			fs := manager.GetFilesystem()

			callbackGet := func(processed int64, total int64) {
				job.Progress(processed, total, false)
			}

			job.Progress(0, sourceEntry.Size, false)

			logger.Debugf("downloading a data object %s to %s", sourcePath, targetFilePath)
			err := fs.DownloadFileParallel(sourcePath, "", targetFilePath, 0, callbackGet)
			if err != nil {
				job.Progress(-1, sourceEntry.Size, true)
				return err
			}

			logger.Debugf("downloaded a data object %s to %s", sourcePath, targetFilePath)
			job.Progress(sourceEntry.Size, sourceEntry.Size, false)
			return nil
		}

		if exist {
			if diff {
				if noHash {
					if targetEntry.Size() == sourceEntry.Size {
						fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
						return nil
					}
				} else {
					if targetEntry.Size() == sourceEntry.Size {
						if len(sourceEntry.CheckSum) > 0 {
							// compare hash
							md5hash, err := commons.HashLocalFileMD5(targetFilePath)
							if err != nil {
								return err
							}

							if sourceEntry.CheckSum == md5hash {
								fmt.Printf("skip downloading a data object %s. The file with the same hash already exists!\n", targetFilePath)
								return nil
							}
						}
					}
				}

				logger.Debugf("deleting an existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return err
				}
			} else if force {
				logger.Debugf("deleting an existing file %s", targetFilePath)
				err := os.Remove(targetFilePath)
				if err != nil {
					return err
				}
			} else {
				// ask
				overwrite := commons.InputYN(fmt.Sprintf("file %s already exists. Overwrite?", targetFilePath))
				if overwrite {
					logger.Debugf("deleting an existing file %s", targetFilePath)
					err := os.Remove(targetFilePath)
					if err != nil {
						return err
					}
				} else {
					fmt.Printf("skip downloading a data object %s. The file already exists!\n", targetFilePath)
					return nil
				}
			}
		}

		threadsRequired := computeThreadsRequiredForGet(sourceEntry.Size)
		parallelJobManager.Schedule(sourcePath, getTask, threadsRequired, progress.UnitsBytes)
		logger.Debugf("scheduled a data object download %s to %s", sourcePath, targetFilePath)
	} else {
		// dir
		logger.Debugf("downloading a collection %s to %s", sourcePath, targetPath)

		entries, err := commons.ListIRODSDir(filesystem, sourceEntry.Path)
		if err != nil {
			return err
		}

		// make target dir
		targetDir := filepath.Join(targetPath, sourceEntry.Name)
		err = os.MkdirAll(targetDir, 0766)
		if err != nil {
			return err
		}

		for idx := range entries {
			path := entries[idx].Path

			err = getOne(parallelJobManager, path, targetDir, force, diff, noHash)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func computeThreadsRequiredForGet(size int64) int {
	// compute num threads required
	threadsRequired := 1
	// 4MB is one thread, max 4 threads
	if size > 4*1024*1024 {
		threadsRequired = int(size / 4 * 1024 * 1024)
		if threadsRequired > 4 {
			threadsRequired = 4
		}
	}

	return threadsRequired
}
