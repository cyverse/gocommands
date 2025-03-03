package commons

import (
	"context"
	"os"
	"runtime"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func CheckNewRelease() (*selfupdate.Release, error) {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "CheckNewVersion",
	})

	logger.Infof("checking latest version for %s/%s", runtime.GOOS, runtime.GOARCH)

	latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug(goCommandsRepoPackagePath))
	if err != nil {
		return nil, xerrors.Errorf("error occurred while detecting version: %w", err)
	}

	if !found {
		return nil, xerrors.Errorf("latest version for %s/%s is not found from github repository %q", runtime.GOOS, runtime.GOARCH, goCommandsRepoPackagePath)
	}

	return latest, nil
}

func SelfUpgrade(release *selfupdate.Release) error {
	logger := log.WithFields(log.Fields{
		"package":  "commons",
		"function": "SelfUpgrade",
	})

	logger.Infof("updating to version v%s, url=%s, name=%s", release.Version(), release.AssetURL, release.AssetName)

	exe, err := os.Executable()
	if err != nil {
		return xerrors.New("failed to locate executable path")
	}

	if err := selfupdate.UpdateTo(context.Background(), release.AssetURL, release.AssetName, exe); err != nil {
		return xerrors.Errorf("error occurred while updating binary: %w", err)
	}

	logger.Infof("updated to version v%s successfully", release.Version())
	return nil
}
