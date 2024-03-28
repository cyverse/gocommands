package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_irodsfs "github.com/cyverse/go-irodsclient/irods/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var mkdirCmd = &cobra.Command{
	Use:     "mkdir [collection1] [collection2] ...",
	Aliases: []string{"imkdir"},
	Short:   "Make iRODS collections",
	Long:    `This makes iRODS collections.`,
	RunE:    processMkdirCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddMkdirCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(mkdirCmd, false)

	flag.SetParentsFlags(mkdirCmd)

	rootCmd.AddCommand(mkdirCmd)
}

func processMkdirCommand(command *cobra.Command, args []string) error {
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

	parentsFlagValues := flag.GetParentsFlagValues()

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	for _, targetPath := range args {
		err = makeOne(filesystem, targetPath, parentsFlagValues)
		if err != nil {
			return xerrors.Errorf("failed to perform mkdir %s: %w", targetPath, err)
		}
	}
	return nil
}

func makeOne(fs *irodsclient_fs.FileSystem, targetPath string, parentsFlagValues *flag.ParentsFlagValues) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "makeOne",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	connection, err := fs.GetMetadataConnection()
	if err != nil {
		return xerrors.Errorf("failed to get connection: %w", err)
	}
	defer fs.ReturnMetadataConnection(connection)

	logger.Debugf("making a collection %s", targetPath)

	err = irodsclient_irodsfs.CreateCollection(connection, targetPath, parentsFlagValues.MakeParents)
	if err != nil {
		return xerrors.Errorf("failed to create collection %s: %w", targetPath, err)
	}
	return nil
}
