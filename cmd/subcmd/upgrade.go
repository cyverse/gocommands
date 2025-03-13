package subcmd

import (
	"runtime"

	"github.com/cyverse/gocommands/cmd/flag"
	"github.com/cyverse/gocommands/commons"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/xerrors"
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

func (upgrade *UpgradeCommand) Process() error {
	cont, err := flag.ProcessCommonFlags(upgrade.command)
	if err != nil {
		return xerrors.Errorf("failed to process common flags: %w", err)
	}

	if !cont {
		return nil
	}

	err = upgrade.upgrade(upgrade.checkVersionFlagValues.Check)
	if err != nil {
		return xerrors.Errorf("failed to upgrade to new release: %w", err)
	}

	return nil
}

func (upgrade *UpgradeCommand) upgrade(checkOnly bool) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"struct":   "UpgradeCommand",
		"function": "upgrade",
	})

	myVersion := commons.GetClientVersion()
	logger.Infof("Current cilent version installed: %s\n", myVersion)
	commons.Printf("Current cilent version installed: %s\n", myVersion)

	newRelease, err := commons.CheckNewRelease()
	if err != nil {
		return xerrors.Errorf("failed to check new release: %w", err)
	}

	logger.Infof("Latest release version available for %s/%s: v%s\n", runtime.GOOS, runtime.GOARCH, newRelease.Version())
	logger.Infof("Latest release URL: %s\n", newRelease.URL)
	commons.Printf("Latest release version available for %s/%s: v%s\n", runtime.GOOS, runtime.GOARCH, newRelease.Version())
	commons.Printf("Latest release URL: %s\n", newRelease.URL)

	if upgrade.hasNewRelease(myVersion, newRelease.Version()) {
		logger.Infof("Need upgrading to latest version v%s\n", newRelease.Version())
		commons.Printf("Need upgrading to latest version v%s\n", newRelease.Version())
	} else {
		logger.Infof("Current client version installed is up-to-date [%s]\n", myVersion)
		commons.Printf("Current client version installed is up-to-date [%s]\n", myVersion)
		return nil
	}

	if checkOnly {
		return nil
	}

	commons.Printf("Upgrading to latest version v%s\n", newRelease.Version())

	err = commons.SelfUpgrade(newRelease)
	if err != nil {
		return xerrors.Errorf("failed to upgrade to new release: %w", err)
	}

	commons.Printf("Upgraded successfully! [%s => v%s]\n", myVersion, newRelease.Version())
	return nil
}

func (upgrade *UpgradeCommand) hasNewRelease(myVersion string, latestVersion string) bool {
	mv1, mv2, mv3 := commons.GetVersionParts(myVersion)
	lv1, lv2, lv3 := commons.GetVersionParts(latestVersion)

	myVersionParts := []int{mv1, mv2, mv3}
	latestVersionParts := []int{lv1, lv2, lv3}

	return commons.IsNewerVersion(latestVersionParts, myVersionParts)
}
