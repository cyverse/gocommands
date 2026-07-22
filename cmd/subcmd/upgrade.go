package subcmd

import (
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/cyverse/gocommands/commons/terminal"
	"github.com/cyverse/gocommands/commons/upgrade"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Gocommands to the latest available version",
	Long:  `This command upgrades Gocommands to the latest version available.`,
	RunE:  processUpgradeCommand,
	Args:  cobra.NoArgs,
}

func AddUpgradeCommand(rootCmd *cobra.Command) {
	// attach common flags
	flag.SetCommonFlags(upgradeCmd, true)

	flag.SetCheckVersionFlags(upgradeCmd)

	rootCmd.AddCommand(upgradeCmd)
}

func processUpgradeCommand(command *cobra.Command, args []string) error {
	upgrade, err := NewUpgradeCommand(command, args)
	if err != nil {
		return err
	}

	return upgrade.Process()
}

type UpgradeCommand struct {
	command *cobra.Command

	commonFlagValues       *flag.CommonFlagValues
	checkVersionFlagValues *flag.CheckVersionFlagValues
}

func NewUpgradeCommand(command *cobra.Command, args []string) (*UpgradeCommand, error) {
	upgrade := &UpgradeCommand{
		command: command,

		commonFlagValues:       flag.GetCommonFlagValues(command),
		checkVersionFlagValues: flag.GetCheckVersionFlagValues(),
	}

	return upgrade, nil
}

func (up *UpgradeCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(up.command)
	if err != nil {
		return errors.Wrapf(err, "failed to process common flags")
	}

	if !cont {
		return nil
	}

	err = up.upgrade(up.checkVersionFlagValues.Check, up.commonFlagValues.Quiet)
	if err != nil {
		return errors.Wrapf(err, "failed to upgrade to new release")
	}

	return nil
}

func (up *UpgradeCommand) upgrade(checkOnly bool, quiet bool) error {
	logger := log.WithFields(log.Fields{
		"check_only": checkOnly,
	})

	myVersion := commons.GetClientVersion()
	if !quiet {
		logger.Infof("Current client version installed: %s", myVersion)
		terminal.Printf("Current client version installed: %s\n", myVersion)
	}

	newRelease, err := upgrade.CheckNewRelease()
	if err != nil {
		return errors.Wrapf(err, "failed to check new release")
	}

	if !quiet {
		logger.Infof("Latest release version available for %s/%s: v%s", runtime.GOOS, runtime.GOARCH, newRelease.Version())
		logger.Infof("Latest release URL: %s", newRelease.URL)
		terminal.Printf("Latest release version available for %s/%s: v%s\n", runtime.GOOS, runtime.GOARCH, newRelease.Version())
		terminal.Printf("Latest release URL: %s\n", newRelease.URL)
	}

	if commons.HasNewRelease(myVersion, newRelease.Version()) {
		if !quiet {
			logger.Infof("Found a new version available: v%s", newRelease.Version())
			terminal.Printf("Found a new version available: v%s\n", newRelease.Version())
		}
	} else {
		if !quiet {
			logger.Infof("Current client version installed is up-to-date: v%s", myVersion)
			terminal.Printf("Current client version installed is up-to-date: v%s\n", myVersion)
		}
		return nil
	}

	if checkOnly {
		return nil
	}

	if !quiet {
		terminal.Printf("Upgrading to latest version: v%s\n", newRelease.Version())
	}

	err = upgrade.SelfUpgrade(newRelease)
	if err != nil {
		return errors.Wrapf(err, "failed to upgrade to the new release")
	}

	if !quiet {
		terminal.Printf("Upgrade from v%s to v%s has done successfully!\n", myVersion, newRelease.Version())
	}
	return nil
}
