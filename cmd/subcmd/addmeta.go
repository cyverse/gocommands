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
	Use:     "addmeta [attribute name] [attribute value] [attribute unit (optional)]",
	Aliases: []string{"add_meta", "add_metadata"},
	Short:   "Add a metadata",
	Long:    `This adds a metadata to the given collection, data object, user, or a resource.`,
	RunE:    processAddmetaCommand,
	Args:    cobra.RangeArgs(2, 3),
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
	addMeta.attribute = args[0]
	addMeta.value = args[1]
	addMeta.unit = ""
	if len(args) >= 3 {
		addMeta.unit = args[2]
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
	if addMeta.targetObjectFlagValues.PathUpdated {
		err = addMeta.addMetaToPath(addMeta.targetObjectFlagValues.Path, addMeta.attribute, addMeta.value, addMeta.unit)
		if err != nil {
			return err
		}
	} else if addMeta.targetObjectFlagValues.UserUpdated {
		err = addMeta.addMetaToUser(addMeta.targetObjectFlagValues.User, addMeta.attribute, addMeta.value, addMeta.unit)
		if err != nil {
			return err
		}
	} else if addMeta.targetObjectFlagValues.ResourceUpdated {
		err = addMeta.addMetaToResource(addMeta.targetObjectFlagValues.Resource, addMeta.attribute, addMeta.value, addMeta.unit)
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

	err := addMeta.filesystem.AddUserMetadata(username, attribute, value, unit)
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

	err := addMeta.filesystem.AddUserMetadata(resource, attribute, value, unit)
	if err != nil {
		return xerrors.Errorf("failed to add metadata to resource %q (attr %q, value %q, unit %q): %w", resource, attribute, value, unit, err)
	}

	return nil
}
