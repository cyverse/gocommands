package subcmd

import (
	"io"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/irods"
	"github.com/cyverse/gocommands/commons/path"
	"github.com/cyverse/gocommands/commons/terminal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var catCmd = &cobra.Command{
	Use:     "cat <data-object>",
	Aliases: []string{"icat"},
	Short:   "Display the content of an iRODS data object",
	Long:    `This command displays the content of the specified iRODS data object.`,
	RunE:    processCatCommand,
	Args:    cobra.MinimumNArgs(1),
}

func AddCatCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(catCmd, false)

	flag.SetTicketAccessFlags(catCmd)

	rootCmd.AddCommand(catCmd)
}

func processCatCommand(command *cobra.Command, args []string) error {
	cat, err := NewCatCommand(command, args)
	if err != nil {
		return err
	}

	return cat.Process()
}

type CatCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	ticketAccessFlagValues *flag.TicketAccessFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
}

func NewCatCommand(command *cobra.Command, args []string) (*CatCommand, error) {
	cat := &CatCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		ticketAccessFlagValues: flag.GetTicketAccessFlagValues(),
	}

	// path
	cat.sourcePaths = args

	return cat, nil
}

func (cat *CatCommand) Process() error {
	logger := log.WithFields(log.Fields{})

	cont, err := flag.ProcessCommonFlags(cat.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	// handle local flags
	_, err = config.InputMissingFields()
	if err != nil {
		return errors.Wrapf(err, "failed to input missing fields")
	}

	// Create a file system
	cat.account = config.GetSessionConfig().ToIRODSAccount()
	if len(cat.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", cat.ticketAccessFlagValues.Name)
		cat.account.Ticket = cat.ticketAccessFlagValues.Name
	}

	cat.filesystem, err = irods.GetIRODSFSClient(cat.account, false, false)
	if err != nil {
		return errors.Wrapf(err, "failed to get iRODS FS Client")
	}
	defer cat.filesystem.Release()

	if cat.commonFlagValues.TimeoutUpdated {
		irods.UpdateIRODSFSClientTimeout(cat.filesystem, cat.commonFlagValues.Timeout)
	}

	// run
	for _, sourcePath := range cat.sourcePaths {
		err = cat.catOne(sourcePath)
		if err != nil {
			return errors.Wrapf(err, "failed to display content of %q", sourcePath)
		}
	}

	return nil
}

func (cat *CatCommand) catOne(sourcePath string) error {
	cwd := config.GetCWD()
	home := config.GetHomeDir()
	zone := cat.account.ClientZone
	sourcePath = path.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := cat.filesystem.Stat(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "failed to stat %q", sourcePath)
	}

	if sourceEntry.IsDir() {
		return errors.Errorf("cannot show the content of a collection %q", sourcePath)
	}

	// file
	fh, err := cat.filesystem.OpenFile(sourcePath, "", "r")
	if err != nil {
		return errors.Wrapf(err, "failed to open file %q", sourcePath)
	}
	defer fh.Close()

	buf := make([]byte, 10240) // 10KB buffer
	for {
		readLen, err := fh.Read(buf)
		if readLen > 0 {
			terminal.Printf("%s", string(buf[:readLen]))
		}

		if err == io.EOF {
			// EOF
			break
		}
	}

	return nil
}
