package upgrade

import (
	"context"
	"os"
	"runtime"

	selfupdate "github.com/creativeprojects/go-selfupdate"
	"github.com/cyverse/gocommands/commons/constant"
	log "github.com/sirupsen/logrus"
	"golang.org/x/xerrors"
)

func CheckNewRelease() (*selfupdate.Release, error) {
	logger := log.WithFields(log.Fields{
		"GOOS":       runtime.GOOS,
		"GOARCH":     runtime.GOARCH,
		"GithubRepo": constant.GoCommandsRepoPackagePath,
	})

	logger.Infof("checking latest version")

	latest, found, err := selfupdate.DetectLatest(context.Background(), selfupdate.ParseSlug(constant.GoCommandsRepoPackagePath))
	if err != nil {
		return nil, xerrors.Errorf("error occurred while detecting version: %w", err)
	}

	if !found {
		return nil, xerrors.Errorf("latest version is not found from github repository")
	}

	return latest, nil
}

func SelfUpgrade(release *selfupdate.Release) error {
	logger := log.WithFields(log.Fields{
		"version":    release.Version(),
		"asset_url":  release.AssetURL,
		"asset_name": release.AssetName,
	})

	logger.Info("updating")

	exe, err := os.Executable()
	if err != nil {
		return xerrors.New("failed to locate executable path")
	}

	if err := selfupdate.UpdateTo(context.Background(), release.AssetURL, release.AssetName, exe); err != nil {
		return xerrors.Errorf("error occurred while updating binary: %w", err)
	}

	logger.Info("updated successfully")
	return nil
}
