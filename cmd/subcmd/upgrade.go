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
	cont, err := flag.ProcessCommonFlags(command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	checkVersionFlagValues := flag.GetCheckVersionFlagValues()

	if checkVersionFlagValues.Check {
		err = checkNewVersion()
		if err != nil {
			return xerrors.Errorf("failed to check new release: %w", err)
		}
	} else {
		err = upgradeToNewVersion()
		if err != nil {
			return xerrors.Errorf("failed to upgrade to new release: %w", err)
		}
	}

	return nil
}

func checkNewVersion() error {
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

func upgradeToNewVersion() error {
	return commons.SelfUpgrade()
}
