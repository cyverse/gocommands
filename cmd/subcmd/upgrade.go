package subcmd

import (
	"fmt"
	"runtime"

	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Gocommands to the latest version available",
	Long:  `This upgrades Gocommands to the latest version available.`,
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

	checkVersionFlagValues *flag.CheckVersionFlagValues
}

func NewUpgradeCommand(command *cobra.Command, args []string) (*UpgradeCommand, error) {
	upgrade := &UpgradeCommand{
		command: command,

		checkVersionFlagValues: flag.GetCheckVersionFlagValues(),
	}

	return upgrade, nil
}

func (upgrade *UpgradeCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(upgrade.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	if upgrade.checkVersionFlagValues.Check {
		err = upgrade.checkNewVersion()
		if err != nil {
			return xerrors.Errorf("failed to check new release: %w", err)
		}

		return nil
	}

	err = upgrade.upgrade()
	if err != nil {
		return xerrors.Errorf("failed to upgrade to new release: %w", err)
	}

	return nil
}

func (upgrade *UpgradeCommand) checkNewVersion() error {
	newRelease, err := commons.CheckNewRelease()
	if err != nil {
		return err
	}

	fmt.Printf("Latest version v%s for %s/%s\n", newRelease.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Latest release URL: %s\n", newRelease.URL)

	myVersion := commons.GetClientVersion()
	fmt.Printf("Current cilent version installed: %s\n", myVersion)
	return nil
}

func (upgrade *UpgradeCommand) upgrade() error {
	return commons.SelfUpgrade()
}
