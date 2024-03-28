package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var addmetaCmd = &cobra.Command{
	Use:     "addmeta [attribute name] [attribute value] [attribute unit (optional)]",
	Aliases: []string{"add_meta", "add_metadata"},
	Short:   "Add a metadata",
	Long:    `This adds a metadata to the given collection, data object, user, or a resource.`,
	RunE:    processAddmetaCommand,
	Args:    cobra.RangeArgs(2, 3),
}

func AddAddmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(addmetaCmd, true)

	flag.SetTargetObjectFlags(addmetaCmd)

	rootCmd.AddCommand(addmetaCmd)
}

func processAddmetaCommand(command *cobra.Command, args []string) error {
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

	targetObjectFlagValues := flag.GetTargetObjectFlagValues(command)

	// Create a connection
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	// get avu
	attr := args[0]
	value := args[1]
	unit := ""
	if len(args) >= 3 {
		unit = args[2]
	}

	if targetObjectFlagValues.PathUpdated {
		err = addMetaToPath(filesystem, targetObjectFlagValues.Path, attr, value, unit)
		if err != nil {
			return err
		}
	} else if targetObjectFlagValues.UserUpdated {
		err = addMetaToUser(filesystem, targetObjectFlagValues.User, attr, value, unit)
		if err != nil {
			return err
		}
	} else if targetObjectFlagValues.ResourceUpdated {
		err = addMetaToResource(filesystem, targetObjectFlagValues.Resource, attr, value, unit)
		if err != nil {
			return err
		}
	} else {
		// nothing updated
		return xerrors.Errorf("path, user, or resource must be given")
	}

	return nil
}

func addMetaToPath(fs *irodsclient_fs.FileSystem, targetPath string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "addMetaToPath",
	})

	logger.Debugf("add metadata to path %s (attr %s, value %s, unit %s)", targetPath, attribute, value, unit)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := fs.AddMetadata(targetPath, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to path %s (attr %s, value %s, unit %s): %w", targetPath, attribute, value, unit, err)
	}

	return nil
}

func addMetaToUser(fs *irodsclient_fs.FileSystem, username string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "addMetaToUser",
	})

	logger.Debugf("add metadata to user %s (attr %s, value %s, unit %s)", username, attribute, value, unit)

	err := fs.AddUserMetadata(username, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to user %s (attr %s, value %s, unit %s): %w", username, attribute, value, unit, err)
	}

	return nil
}

func addMetaToResource(fs *irodsclient_fs.FileSystem, resource string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "addMetaToResource",
	})

	logger.Debugf("add metadata to resource %s (attr %s, value %s, unit %s)", resource, attribute, value, unit)

	err := fs.AddUserMetadata(resource, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to resource %s (attr %s, value %s, unit %s): %w", resource, attribute, value, unit, err)
	}

	return nil
}
