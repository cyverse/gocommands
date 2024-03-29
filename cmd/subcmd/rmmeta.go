package subcmd

import (
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rmmetaCmd = &cobra.Command{
	Use:     "rmmeta [AVU ID|attribute name] ...",
	Aliases: []string{"rm_meta", "remove_meta", "rm_metadata", "remove_metadata", "delete_meta", "delete_metadata"},
	Short:   "Remove metadatas for the user",
	Long:    `This removes metadata of the given collection, data object, user, or a resource.`,
	RunE:    processRmmetaCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddRmmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(rmmetaCmd, true)

	flag.SetTargetObjectFlags(rmmetaCmd)

	rootCmd.AddCommand(rmmetaCmd)
}

func processRmmetaCommand(command *cobra.Command, args []string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "processRmmetaCommand",
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

	targetObjectFlagValues := flag.GetTargetObjectFlagValues(command)

	// Create a file system
	account := commons.GetAccount()
	filesystem, err := commons.GetIRODSFSClient(account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}

	defer filesystem.Release()

	for _, avuidString := range args {
		if commons.IsDigitsOnly(avuidString) {
			// avuid
			avuid, err := strconv.ParseInt(avuidString, 10, 64)
			if err != nil {
				return xerrors.Errorf("failed to parse AVUID: %w", err)
			}

			if targetObjectFlagValues.PathUpdated {
				err = removeMetaFromPath(filesystem, targetObjectFlagValues.Path, avuid)
				if err != nil {
					return err
				}
			} else if targetObjectFlagValues.UserUpdated {
				err = removeMetaFromUser(filesystem, targetObjectFlagValues.User, avuid)
				if err != nil {
					return err
				}
			} else if targetObjectFlagValues.ResourceUpdated {
				err = removeMetaFromResource(filesystem, targetObjectFlagValues.Resource, avuid)
				if err != nil {
					return err
				}
			} else {
				// nothing updated
				return xerrors.Errorf("path, user, or resource must be given")
			}
		} else {
			// possibly name
			attr := avuidString
			logger.Debugf("remove metadata with name %s", attr)

			if targetObjectFlagValues.PathUpdated {
				err = removeMetaFromPathByName(filesystem, targetObjectFlagValues.Path, attr)
				if err != nil {
					return err
				}
			} else if targetObjectFlagValues.UserUpdated {
				err = removeMetaFromUserByName(filesystem, targetObjectFlagValues.User, attr)
				if err != nil {
					return err
				}
			} else if targetObjectFlagValues.ResourceUpdated {
				err = removeMetaFromResourceByName(filesystem, targetObjectFlagValues.Resource, attr)
				if err != nil {
					return err
				}
			} else {
				// nothing updated
				return xerrors.Errorf("path, user, or resource must be given")
			}
		}

	}
	return nil
}

func removeMetaFromPath(fs *irodsclient_fs.FileSystem, targetPath string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromPath",
	})

	logger.Debugf("remove metadata %d from path %s", avuid, targetPath)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := fs.DeleteMetadata(targetPath, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from path %s: %w", avuid, targetPath, err)
	}

	return nil
}

func removeMetaFromUser(fs *irodsclient_fs.FileSystem, username string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromUser",
	})

	logger.Debugf("remove metadata %d from user %s", avuid, username)

	err := fs.DeleteUserMetadata(username, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from user %s: %w", avuid, username, err)
	}

	return nil
}

func removeMetaFromResource(fs *irodsclient_fs.FileSystem, resource string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromResource",
	})

	logger.Debugf("remove metadata %d from resource %s", avuid, resource)

	err := fs.DeleteResourceMetadata(resource, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from resource %s: %w", avuid, resource, err)
	}

	return nil
}

func removeMetaFromPathByName(fs *irodsclient_fs.FileSystem, targetPath string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromPathByName",
	})

	logger.Debugf("remove metadata %s from path %s by name", attr, targetPath)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := commons.GetZone()
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := fs.DeleteMetadataByName(targetPath, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %s from path %s by name: %w", attr, targetPath, err)
	}

	return nil
}

func removeMetaFromUserByName(fs *irodsclient_fs.FileSystem, username string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromUserByName",
	})

	logger.Debugf("remove metadata %s from user %s by name", attr, username)

	err := fs.DeleteUserMetadataByName(username, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %s from user %s by name: %w", attr, username, err)
	}

	return nil
}

func removeMetaFromResourceByName(fs *irodsclient_fs.FileSystem, resource string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"function": "removeMetaFromResourceByName",
	})

	logger.Debugf("remove metadata %s from resource %s by name", attr, resource)

	err := fs.DeleteResourceMetadataByName(resource, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %s from resource %s by name: %w", attr, resource, err)
	}

	return nil
}
