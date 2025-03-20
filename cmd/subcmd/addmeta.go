package subcmd

import (
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var addmetaCmd = &cobra.Command{
	Use:     "addmeta <irods-object> <metadata-name> <metadata-value> [metadata-unit]",
	Aliases: []string{"add_meta", "add_metadata"},
	Short:   "Add metadata to a specified iRODS object",
	Long:    `This command adds metadata to a specified iRODS object, such as a collection, data object, user, or resource. The metadata consists of a name, value, and optionally a unit.`,
	RunE:    processAddmetaCommand,
	Args:    cobra.RangeArgs(3, 4),
}

func AddAddmetaCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlagsWithoutResource(addmetaCmd)

	flag.SetTargetObjectFlags(addmetaCmd)

	rootCmd.AddCommand(addmetaCmd)
}

func processAddmetaCommand(command *cobra.Command, args []string) error {
	addMeta, err := NewAddMetaCommand(command, args)
	if err != nil {
		return err
	}

	return addMeta.Process()
}

type AddMetaCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	targetObjectFlagValues *flag.TargetObjectFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	targetObject string

	attribute string
	value     string
	unit      string
}

func NewAddMetaCommand(command *cobra.Command, args []string) (*AddMetaCommand, error) {
	addMeta := &AddMetaCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		targetObjectFlagValues: flag.GetTargetObjectFlagValues(command),
	}

	// get avu
	addMeta.targetObject = args[0]
	addMeta.attribute = args[1]
	addMeta.value = args[2]
	addMeta.unit = ""
	if len(args) >= 4 {
		addMeta.unit = args[3]
	}

	return addMeta, nil
}

func (addMeta *AddMetaCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(addMeta.command)
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
	addMeta.account = commons.GetSessionConfig().ToIRODSAccount()
	addMeta.filesystem, err = commons.GetIRODSFSClientForSingleOperation(addMeta.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer addMeta.filesystem.Release()

	// add meta
	if addMeta.targetObjectFlagValues.Path {
		err = addMeta.addMetaToPath(addMeta.targetObject, addMeta.attribute, addMeta.value, addMeta.unit)
		if err != nil {
			return err
		}
	} else if addMeta.targetObjectFlagValues.User {
		err = addMeta.addMetaToUser(addMeta.targetObject, addMeta.attribute, addMeta.value, addMeta.unit)
		if err != nil {
			return err
		}
	} else if addMeta.targetObjectFlagValues.Resource {
		err = addMeta.addMetaToResource(addMeta.targetObject, addMeta.attribute, addMeta.value, addMeta.unit)
		if err != nil {
			return err
		}
	} else {
		// nothing updated
		return xerrors.Errorf("path, user, or resource must be given")
	}

	return nil
}

func (addMeta *AddMetaCommand) addMetaToPath(targetPath string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "AddMetaCommand",
		"function": "addMetaToPath",
	})

	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := addMeta.account.ClientZone
	targetPath = commons.MakeIRODSPath(cwd, home, zone, targetPath)

	logger.Debugf("add metadata to path %q (attr %q, value %q, unit %q)", targetPath, attribute, value, unit)

	err := addMeta.filesystem.AddMetadata(targetPath, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to path %q (attr %q, value %q, unit %q): %w", targetPath, attribute, value, unit, err)
	}

	return nil
}

func (addMeta *AddMetaCommand) addMetaToUser(username string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "AddMetaCommand",
		"function": "addMetaToUser",
	})

	logger.Debugf("add metadata to user %q (attr %q, value %q, unit %q)", username, attribute, value, unit)

	err := addMeta.filesystem.AddUserMetadata(username, addMeta.account.ClientZone, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to user %q (attr %q, value %q, unit %q): %w", username, attribute, value, unit, err)
	}

	return nil
}

func (addMeta *AddMetaCommand) addMetaToResource(resource string, attribute string, value string, unit string) error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "AddMetaCommand",
		"function": "addMetaToResource",
	})

	logger.Debugf("add metadata to resource %q (attr %q, value %q, unit %q)", resource, attribute, value, unit)

	err := addMeta.filesystem.AddUserMetadata(resource, addMeta.account.ClientZone, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to resource %q (attr %q, value %q, unit %q): %w", resource, attribute, value, unit, err)
	}

	return nil
}
