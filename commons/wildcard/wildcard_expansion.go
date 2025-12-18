package wildcard

import (
	"sort"

	"github.com/cockroachdb/errors"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/types"
	irodsclient_util "github.com/cyverse/go-irodsclient/irods/util"
	"github.com/cyverse/gocommands/commons/config"
	"github.com/cyverse/gocommands/commons/path"
)

func ExpandWildcards(fs *irodsclient_fs.FileSystem, account *types.IRODSAccount, targetPaths []string, expandCollections bool, expandDataobjects bool) ([]string, error) {
	if !expandCollections && !expandDataobjects {
		return nil, errors.New("Need to enable data objects or collections (or both) for wildcard expansion.")
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
					return nil, errors.Wrapf(err, "failed to perform collection query for wildcard expansion")
				}

				for _, searchResult := range searchResults {
					outputPaths = append(outputPaths, searchResult.Path)
				}
			}

			if expandDataobjects {
				searchResults, err := fs.SearchFileUnixWildcard(absPath)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to perform data object query for wildcard expansion")
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
