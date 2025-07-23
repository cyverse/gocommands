package wildcard

import (
	"sort"

	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/path"
	"golang.org/x/xerrors"
)

func ExpandWildcards(fs *irodsclient_fs.FileSystem, account *types.IRODSAccount, targetPaths []string, expandCollections bool, expandDataobjects bool) ([]string, error) {
	if !expandCollections && !expandDataobjects {
		return nil, xerrors.Errorf("Need to enable data objects or collections (or both) for wildcard expansion.")
	}

	outputPaths := []string{}

	for _, targetPath := range targetPaths {
		if irodsclient_util.HasWildcards(targetPath) {
			// First convert targetPath to absolute path
			cwd := config.GetCWD()
			home := config.GetHomeDir()
			zone := account.ClientZone
			absPath := path.MakeIRODSPath(cwd, home, zone, targetPath)

			// Perform queries
			if expandCollections {
				searchResults, err := fs.SearchDirUnixWildcard(absPath)
				if err != nil {
					return nil, xerrors.Errorf("failed to perform collection query for wildcard expansion: %w", err)
				}

				for _, searchResult := range searchResults {
					outputPaths = append(outputPaths, searchResult.Path)
				}
			}

			if expandDataobjects {
				searchResults, err := fs.SearchFileUnixWildcard(absPath)
				if err != nil {
					return nil, xerrors.Errorf("failed to perform data object query for wildcard expansion: %w", err)
				}

				for _, searchResult := range searchResults {
					outputPaths = append(outputPaths, searchResult.Path)
				}
			}
		} else {
			outputPaths = append(outputPaths, targetPath)
		}
	}

	// sort outputPaths by path
	sort.SliceStable(outputPaths, func(i, j int) bool {
		return outputPaths[i] < outputPaths[j]
	})

	return outputPaths, nil
}
