package subcmd

import (
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rmmetaCmd = &cobra.Command{
	Use:     "rmmeta <irods-object> <metadata-ID-or-name>...",
	Aliases: []string{"rm_meta", "remove_meta", "rm_metadata", "remove_metadata", "delete_meta", "delete_metadata"},
	Short:   "Remove metadata for a collection, data object, user, or resource",
	Long:    `This command removes metadata from a specified iRODS object, such as a collection, data object, user, or resource.`,
	RunE:    processRmmetaCommand,
	Args:    cobra.MinimumNArgs(2),
}

func AddRmmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlagsWithoutResource(rmmetaCmd)

	flag.SetTargetObjectFlags(rmmetaCmd)

	rootCmd.AddCommand(rmmetaCmd)
}

func processRmmetaCommand(command *cobra.Command, args []string) error {
	rmMeta, err := NewRmMetaCommand(command, args)
	if err != nil {
		return err
	}

	return rmMeta.Process()
}

type RmMetaCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	targetObjectFlagValues *flag.TargetObjectFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetObject string
	avuIDs       []string
}

func NewRmMetaCommand(command *cobra.Command, args []string) (*RmMetaCommand, error) {
	rmMeta := &RmMetaCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		targetObjectFlagValues: flag.GetTargetObjectFlagValues(command),
	}

	// path
	rmMeta.targetObject = args[0]
	rmMeta.avuIDs = args[1:]

	return rmMeta, nil
}

func (rmMeta *RmMetaCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(rmMeta.command)
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
	rmMeta.account = commons.GetSessionConfig().ToIRODSAccount()
	rmMeta.filesystem, err = commons.GetIRODSFSClientForSingleOperation(rmMeta.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer rmMeta.filesystem.Release()

	// remove
	for _, avuidString := range rmMeta.avuIDs {
		err = rmMeta.removeOne(avuidString)
		if err != nil {
			return xerrors.Errorf("failed to remove meta for avuid (or name) %q: %w", avuidString, err)
		}
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeOne(avuidString string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeOne",
	})

	if commons.IsDigitsOnly(avuidString) {
		// avu ID
		avuid, err := strconv.ParseInt(avuidString, 10, 64)
		if err != nil {
			return xerrors.Errorf("failed to parse AVUID: %w", err)
		}

		return rmMeta.removeOneByID(avuid)
	}

	// possibly name
	logger.Debugf("remove metadata with name %q", avuidString)
	return rmMeta.removeOneByName(avuidString)
}

func (rmMeta *RmMetaCommand) removeOneByID(avuID int64) error {
	if rmMeta.targetObjectFlagValues.Path {
		err := rmMeta.removeMetaFromPath(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.User {
		err := rmMeta.removeMetaFromUser(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.Resource {
		err := rmMeta.removeMetaFromResource(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	}

	// nothing updated
	return xerrors.Errorf("one of path, user, or resource must be selected")
}

func (rmMeta *RmMetaCommand) removeOneByName(attrName string) error {
	if rmMeta.targetObjectFlagValues.Path {
		err := rmMeta.removeMetaFromPathByName(rmMeta.targetObject, attrName)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.User {
		err := rmMeta.removeMetaFromUserByName(rmMeta.targetObject, attrName)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.Resource {
		err := rmMeta.removeMetaFromResourceByName(rmMeta.targetObject, attrName)
		if err != nil {
			return err
		}

		return nil
	}

	// nothing updated
	return xerrors.Errorf("one of path, user, or resource must be selected")
}

func (rmMeta *RmMetaCommand) removeMetaFromPath(targetPath string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromPath",
	})

	logger.Debugf("remove metadata %d from path %q", avuid, targetPath)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := rmMeta.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := rmMeta.filesystem.DeleteMetadata(targetPath, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from path %q: %w", avuid, targetPath, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromUser(username string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromUser",
	})

	logger.Debugf("remove metadata %d from user %q", avuid, username)

	err := rmMeta.filesystem.DeleteUserMetadata(username, rmMeta.account.ClientZone, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from user %q: %w", avuid, username, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromResource(resource string, avuid int64) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromResource",
	})

	logger.Debugf("remove metadata %d from resource %q", avuid, resource)

	err := rmMeta.filesystem.DeleteResourceMetadata(resource, avuid)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %d from resource %q: %w", avuid, resource, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromPathByName(targetPath string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromPathByName",
	})

	logger.Debugf("remove metadata %q from path %q by name", attr, targetPath)

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := rmMeta.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	err := rmMeta.filesystem.DeleteMetadataByName(targetPath, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %q from path %q by name: %w", attr, targetPath, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromUserByName(username string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromUserByName",
	})

	logger.Debugf("remove metadata %q from user %q by name", attr, username)

	err := rmMeta.filesystem.DeleteUserMetadataByName(username, rmMeta.account.ClientZone, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %q from user %q by name: %w", attr, username, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromResourceByName(resource string, attr string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "RmMetaCommand",
		"function": "removeMetaFromResourceByName",
	})

	logger.Debugf("remove metadata %q from resource %q by name", attr, resource)

	err := rmMeta.filesystem.DeleteResourceMetadataByName(resource, attr)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata %q from resource %q by name: %w", attr, resource, err)
	}

	return nil
}
