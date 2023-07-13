package subcmd

import (
	"fmt"
	"io"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var catCmd = &cobra.Command{
	Use:     "cat [data-object]",
	Aliases: []string{"icat"},
	Short:   "Display the content of an iRODS data-object",
	Long:    `This displays the content of an iRODS data-object.`,
	RunE:    processCatCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddCatCommand(rootCmd *cobra.Command) {
	// attach common flags
	commons.SetCommonFlags(catCmd)

	rootCmd.AddCommand(catCmd)
}

func processCatCommand(command *cobra.Command, args []string) error {
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

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	for _, sourcePath := range args {
		err = catOne(filesystem, sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to perform cat %s: %w", sourcePath, err)
		}
	}
	return nil
}

func catOne(filesystem *irodsclient_fs.FileSystem, targetPath string) error {
	logger := log.WithFields(log.Fields{
		"package":  "main",
		"function": "catOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	targetEntry, err := commons.StatIRODSPath(filesystem, targetPath)
	if err != nil {
		return xerrors.Errorf("failed to stat %s: %w", targetPath, err)
	}

	if targetEntry.Type == irodsclient_fs.FileEntry {
		// file
		logger.Debugf("showing the content of a data object %s", targetPath)
		fh, err := filesystem.OpenFile(targetPath, "", "r")
		if err != nil {
			return xerrors.Errorf("failed to open file %s: %w", targetPath, err)
		}

		defer fh.Close()

		buf := make([]byte, 10240) // 10KB buffer
		for {
			readLen, err := fh.Read(buf)
			if readLen > 0 {
				fmt.Printf("%s", string(buf[:readLen]))
			}

			if err == io.EOF {
				// EOF
				break
			}
		}

	} else {
		// dir
		return xerrors.Errorf("cannot show the content of a collection")
	}
	return nil
}
