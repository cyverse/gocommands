package subcmd

import (
	"strconv"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var rmmetaCmd = &cobra.Command{
	Use:     "rmmeta <irods-object> <metadata-ID>... OR <irods-object> <metadata-name> <metadata-value> [metadata-unit]",
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
	flag.SetMetadataByIDFlags(rmmetaCmd)

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
	metadataByIDFlagValues *flag.MetadataByIDFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetObject string

	avuIDs []string

	attribute string
	value     string
	unit      string
}

func NewRmMetaCommand(command *cobra.Command, args []string) (*RmMetaCommand, error) {
	rmMeta := &RmMetaCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		targetObjectFlagValues: flag.GetTargetObjectFlagValues(command),
		metadataByIDFlagValues: flag.GetMetadataByIDFlagValues(command),
	}

	// path
	rmMeta.targetObject = args[0]

	if rmMeta.metadataByIDFlagValues.ByID {
		rmMeta.avuIDs = args[1:]
	} else {
		rmMeta.attribute = args[1]
		rmMeta.value = args[2]
		rmMeta.unit = ""
		if len(args) >= 4 {
			rmMeta.unit = args[3]
		}
	}

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
	_, err = config.InputMissingFields()
	if err != nil {
		return xerrors.Errorf("failed to input missing fields: %w", err)
	}

	// Create a file system
	rmMeta.account = config.GetSessionConfig().ToIRODSAccount()
	rmMeta.filesystem, err = irods.GetIRODSFSClient(rmMeta.account, true, false)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer rmMeta.filesystem.Release()

	if rmMeta.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(rmMeta.filesystem, rmMeta.commonFlagValues.Timeout)
	}

	// remove
	if rmMeta.metadataByIDFlagValues.ByID {
		// id
		for _, avuIDString := range rmMeta.avuIDs {
			err = rmMeta.removeOneByID(avuIDString)
			if err != nil {
				return xerrors.Errorf("failed to remove meta for avuID %q: %w", avuIDString, err)
			}
		}
	} else {
		// avu
		err = rmMeta.removeOneByAVU(rmMeta.attribute, rmMeta.value, rmMeta.unit)
		if err != nil {
			return xerrors.Errorf("failed to remove meta for attr %q, val %q, unit %q: %w", rmMeta.attribute, rmMeta.value, rmMeta.unit, err)
		}
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeOneByID(avuIDString string) error {
	// avu ID
	avuID, err := strconv.ParseInt(avuIDString, 10, 64)
	if err != nil {
		return xerrors.Errorf("failed to parse AVUID: %w", err)
	}

	if rmMeta.targetObjectFlagValues.Path {
		err := rmMeta.removeMetaFromPathByID(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.User {
		err := rmMeta.removeMetaFromUserByID(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.Resource {
		err := rmMeta.removeMetaFromResourceByID(rmMeta.targetObject, avuID)
		if err != nil {
			return err
		}

		return nil
	}

	// nothing updated
	return xerrors.Errorf("one of path, user, or resource must be selected")
}

func (rmMeta *RmMetaCommand) removeOneByAVU(attribute string, value string, unit string) error {
	if rmMeta.targetObjectFlagValues.Path {
		err := rmMeta.removeMetaFromPathByAVU(rmMeta.targetObject, attribute, value, unit)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.User {
		err := rmMeta.removeMetaFromUserByAVU(rmMeta.targetObject, attribute, value, unit)
		if err != nil {
			return err
		}

		return nil
	} else if rmMeta.targetObjectFlagValues.Resource {
		err := rmMeta.removeMetaFromResourceByAVU(rmMeta.targetObject, attribute, value, unit)
		if err != nil {
			return err
		}

		return nil
	}

	// nothing updated
	return xerrors.Errorf("one of path, user, or resource must be selected")
}

func (rmMeta *RmMetaCommand) removeMetaFromPathByID(targetPath string, avuID int64) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
		"avu_id":      avuID,
	})

	logger.Debug("remove metadata from path")

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := rmMeta.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	err := rmMeta.filesystem.DeleteMetadata(targetPath, avuID)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (id: %d) from path %q: %w", avuID, targetPath, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromUserByID(username string, avuID int64) error {
	logger := log.WithFields(log.Fields{
		"username": username,
		"avu_id":   avuID,
	})

	logger.Debug("remove metadata from user")

	err := rmMeta.filesystem.DeleteUserMetadata(username, rmMeta.account.ClientZone, avuID)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (id: %d) from user %q: %w", avuID, username, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromResourceByID(resource string, avuID int64) error {
	logger := log.WithFields(log.Fields{
		"resource": resource,
		"avu_id":   avuID,
	})

	logger.Debug("remove metadata from resource")

	err := rmMeta.filesystem.DeleteResourceMetadata(resource, avuID)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (id: %d) from resource %q: %w", avuID, resource, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromPathByAVU(targetPath string, attr string, val string, unit string) error {
	logger := log.WithFields(log.Fields{
		"target_path": targetPath,
		"attr":        attr,
		"val":         val,
		"unit":        unit,
	})

	logger.Debug("remove metadata from path")

	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := rmMeta.account.ClientZone
	targetPath = path.MakeIRODSPath(cwd, home, zone, targetPath)

	err := rmMeta.filesystem.DeleteMetadataByAVU(targetPath, attr, val, unit)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (attr: %q, val: %q, unit: %q) from path %q: %w", attr, val, unit, targetPath, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromUserByAVU(username string, attr string, val string, unit string) error {
	logger := log.WithFields(log.Fields{
		"username": username,
		"attr":     attr,
		"val":      val,
		"unit":     unit,
	})

	logger.Debug("remove metadata from user")

	err := rmMeta.filesystem.DeleteUserMetadataByAVU(username, rmMeta.account.ClientZone, attr, val, unit)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (attr: %q, val: %q, unit: %q) from user %q: %w", attr, val, unit, username, err)
	}

	return nil
}

func (rmMeta *RmMetaCommand) removeMetaFromResourceByAVU(resource string, attr string, val string, unit string) error {
	logger := log.WithFields(log.Fields{
		"resource": resource,
		"attr":     attr,
		"val":      val,
		"unit":     unit,
	})

	logger.Debug("remove metadata from resource")

	err := rmMeta.filesystem.DeleteResourceMetadataByAVU(resource, attr, val, unit)
	if err != nil {
		return xerrors.Errorf("failed to delete metadata (attr: %q, val: %q, unit: %q) from resource %q: %w", attr, val, unit, resource, err)
	}

	return nil
}
