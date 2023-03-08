package commons

import (
	"context"
	"os"
	"runtime"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func SelfUpdate() error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "SelfUpdate",
	})

	logger.Infof("checking latest version for %s/%s", runtime.GOOS, runtime.GOARCH)

	latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug("cyverse/gocommands"))
	if err != nil {
		return xerrors.Errorf("error occurred while detecting version: %w", err)
	}

	if !found {
		return xerrors.Errorf("latest version for %s/%s could not be found from github repository 'cyverse/gocommands'", runtime.GOOS, runtime.GOARCH)
	}

	myVersion := GetClientVersion()
	logger.Infof("found latest version v%s for %s/%s (my version = %s)", latest.Version(), runtime.GOOS, runtime.GOARCH, myVersion)

	mv1, mv2, mv3 := GetVersionParts(myVersion)
	lv1, lv2, lv3 := GetVersionParts(latest.Version())

	myVersionParts := []int{mv1, mv2, mv3}
	latestVersionParts := []int{lv1, lv2, lv3}

	if !IsNewerVersion(latestVersionParts, myVersionParts) {
		logger.Infof("you already have the latest version %s", myVersion)
		return nil
	}

	logger.Infof("updating to latest version v%s, url=%s, name=%s", latest.Version(), latest.AssetURL, latest.AssetName)

	exe, err := os.Executable()
	if err != nil {
		return xerrors.New("failed to locate executable path")
	}

	if err := selfupdate.UpdateTo(context.Background(), latest.AssetURL, latest.AssetName, exe); err != nil {
		return xerrors.Errorf("error occurred while updating binary: %w", err)
	}

	logger.Infof("updated to version v%s successfully", latest.Version())
	return nil
}
