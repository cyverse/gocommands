package subcmd

import (
	"io"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/cmd/flag"
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

	ticketAccessFlagValues *flag.TicketAccessFlagValues

	account    *irodsclient_types.IRODSAccount
	filesystem *irodsclient_fs.FileSystem

	sourcePaths []string
}

func NewCatCommand(command *cobra.Command, args []string) (*CatCommand, error) {
	cat := &CatCommand{
		command: command,

		ticketAccessFlagValues: flag.GetTicketAccessFlagValues(),
	}

	// path
	cat.sourcePaths = args

	return cat, nil
}

func (cat *CatCommand) Process() error {
	logger := log.WithFields(log.Fields{
		"package":  "subcmd",
		"struct":   "CatCommand",
		"function": "Process",
	})

	cont, err := flag.ProcessCommonFlags(cat.command)
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
	cat.account = commons.GetSessionConfig().ToIRODSAccount()
	if len(cat.ticketAccessFlagValues.Name) > 0 {
		logger.Debugf("use ticket: %q", cat.ticketAccessFlagValues.Name)
		cat.account.Ticket = cat.ticketAccessFlagValues.Name
	}

	cat.filesystem, err = commons.GetIRODSFSClientForSingleOperation(cat.account)
	if err != nil {
		return xerrors.Errorf("failed to get iRODS FS Client: %w", err)
	}
	defer cat.filesystem.Release()

	// run
	for _, sourcePath := range cat.sourcePaths {
		err = cat.catOne(sourcePath)
		if err != nil {
			return xerrors.Errorf("failed to display content of %q: %w", sourcePath, err)
		}
	}

	return nil
}

func (cat *CatCommand) catOne(sourcePath string) error {
	cwd := commons.GetCWD()
	home := commons.GetHomeDir()
	zone := cat.account.ClientZone
	sourcePath = commons.MakeIRODSPath(cwd, home, zone, sourcePath)

	sourceEntry, err := cat.filesystem.Stat(sourcePath)
	if err != nil {
		return xerrors.Errorf("failed to stat %q: %w", sourcePath, err)
	}

	if sourceEntry.IsDir() {
		return xerrors.Errorf("cannot show the content of a collection")
	}

	// file
	fh, err := cat.filesystem.OpenFile(sourcePath, "", "r")
	if err != nil {
		return xerrors.Errorf("failed to open file %q: %w", sourcePath, err)
	}
	defer fh.Close()

	buf := make([]byte, 10240) // 10KB buffer
	for {
		readLen, err := fh.Read(buf)
		if readLen > 0 {
			commons.Printf("%s", string(buf[:readLen]))
		}

		if err == io.EOF {
			// EOF
			break
		}
	}

	return nil
}
