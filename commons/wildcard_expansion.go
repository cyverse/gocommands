package commons

import (
	"fmt"
	irodsclient_fs "github.com/cyverse/go-irodsclient/fs"
	"github.com/cyverse/go-irodsclient/irods/common"
	"github.com/cyverse/go-irodsclient/irods/connection"
	"github.com/cyverse/go-irodsclient/irods/message"
	"github.com/cyverse/go-irodsclient/irods/types"
	"github.com/danwakefield/fnmatch"
	"github.com/dlclark/regexp2"
	"golang.org/x/xerrors"
	"regexp"
	"strings"
)

func ExpandWildcards(fs *irodsclient_fs.FileSystem, account *types.IRODSAccount, input []string, expand_collections bool, expand_dataobjects bool) ([]string, error) {

	if !expand_collections && !expand_dataobjects {
		return nil, xerrors.Errorf("Need to enable data objects or collections (or both) for wildcard expansion.")
	}
	output := make([]string, 0)

	conn, err_connection := fs.GetMetadataConnection()
	if err_connection != nil {
		return nil, xerrors.Errorf("failed to get connection: %w", err_connection)
	}
	defer fs.ReturnMetadataConnection(conn)

	for i := 0; i < len(input); i++ {
		if hasWildcards(input[i]) {
			// First convert input to absolute path
			cwd := GetCWD()
			home := GetHomeDir()
			zone := account.ClientZone
			absolute_path := MakeIRODSPath(cwd, home, zone, input[i])

			// Convert Unix wildcards into strings for SQL queries
			absolute_path_sql_wildcard := unixWildcardsToSQLWildcards(absolute_path)
			basename_sql_wildcard := getIRODSPathBasename(absolute_path_sql_wildcard)
			dirname_sql_wildcard := getIRODSPathDirname(absolute_path_sql_wildcard)

			// Perform queries
			query_results := make([]string, 0)
			if expand_collections {
				coll_query_results, err := wildcardSearchCollection(conn, absolute_path_sql_wildcard)
				if err != nil {
					return nil, xerrors.Errorf("failed to perform collection query for wildcard expansion: %w", err)
				}
				query_results = append(query_results, (*coll_query_results)...)
			}
			if expand_dataobjects {
				do_query_results, err := wildcardSearchDataObject(conn, dirname_sql_wildcard, basename_sql_wildcard)
				if err != nil {
					return nil, xerrors.Errorf("failed to perform data object query for wildcard expansion: %w", err)
				}
				query_results = append(query_results, (*do_query_results)...)
			}

			// Add results to output. Filter results by original unix wildcard, since the SQL wildcards
			// are less strict (e.g. a unix wildcard range is converted to a generic
			// wildcards in SQL).

			for j := 0; j < len(query_results); j++ {
				if fnmatch.Match(absolute_path, query_results[j], fnmatch.FNM_PATHNAME) {
					output = append(output, query_results[j])
				}
			}

		} else {
			output = append(output, input[i])
		}
	}
	return output, nil
}

func hasWildcards(input string) bool {
	return (regexp.MustCompile(`(?:[^\\])(?:\\\\)*[?*]`).MatchString(input) ||
		regexp.MustCompile(`^(?:\\\\)*[?*]`).MatchString(input) ||
		regexp.MustCompile(`(?:[^\\])(?:\\\\)*\[.*?(?:[^\\])(?:\\\\)*\]`).MatchString(input) ||
		regexp.MustCompile(`^(?:\\\\)*\[.*?(?:[^\\])(?:\\\\)*\]`).MatchString(input))
}

func unixWildcardsToSQLWildcards(input string) string {
	output := input
	length := len(input)
	// Use regexp2 rather than regexp here in order to be able to use lookbehind assertions
	//
	// Escape SQL wildcard characters
	output = strings.ReplaceAll(output, "%", `\%`)
	output = strings.ReplaceAll(output, "_", `\_`)
	// Replace ranges with a wildcard
	output, _ = regexp2.MustCompile(`(?<!\\)(?:\\\\)*\[.*?(?<!\\)(?:\\\\)*\]`, regexp2.RE2).Replace(output, `_`, 0, length)
	// Replace non-escaped regular wildcard characters with SQL equivalents
	output, _ = regexp2.MustCompile(`(?<!\\)(?:\\\\)*(\*)`, regexp2.RE2).Replace(output, `%`, 0, length)
	output, _ = regexp2.MustCompile(`(?<!\\)(?:\\\\)*(\?)`, regexp2.RE2).Replace(output, `_`, 0, length)
	return output
}

func wildcardSearchCollection(conn *connection.IRODSConnection, collection_wildcard_value string) (*[]string, error) {
	if conn == nil || !conn.IsConnected() {
		return nil, xerrors.Errorf("connection is nil or disconnected")
	}

	// lock the connection
	conn.Lock()
	defer conn.Unlock()

	continueQuery := true
	continueIndex := 0
	results := make([]string, 0)

	for continueQuery {
		query := message.NewIRODSMessageQueryRequest(common.MaxQueryRows, continueIndex, 0, 0)
		query.AddKeyVal(common.ZONE_KW, conn.GetAccount().ClientZone)
		query.AddSelect(common.ICAT_COLUMN_COLL_NAME, 1)

		wildcard_condition := fmt.Sprintf("LIKE '%s'", collection_wildcard_value)
		query.AddCondition(common.ICAT_COLUMN_COLL_NAME, wildcard_condition)

		queryResult := message.IRODSMessageQueryResponse{}
		err := conn.Request(query, &queryResult, nil)
		if err != nil {
			return nil, xerrors.Errorf("failed to receive a collection query result message: %w", err)
		}

		err = queryResult.CheckError()
		if err != nil {
			if types.GetIRODSErrorCode(err) == common.CAT_NO_ROWS_FOUND {
				// empty
				break
			}
			return nil, xerrors.Errorf("received collection query error: %w", err)
		}

		if queryResult.RowCount == 0 {
			break
		}

		sqlResult := queryResult.SQLResult[0]
		for row := 0; row < queryResult.RowCount; row++ {
			results = append(results, sqlResult.Values[row])
		}

		continueIndex = queryResult.ContinueIndex
		if continueIndex == 0 {
			continueQuery = false
		}
	}

	return &results, nil

}

func wildcardSearchDataObject(conn *connection.IRODSConnection, collection_wildcard_value string, dataobject_wildcard_value string) (*[]string, error) {
	if conn == nil || !conn.IsConnected() {
		return nil, xerrors.Errorf("connection is nil or disconnected")
	}

	// lock the connection
	conn.Lock()
	defer conn.Unlock()

	continueQuery := true
	continueIndex := 0
	results := make([]string, 0)

	for continueQuery {
		query := message.NewIRODSMessageQueryRequest(common.MaxQueryRows, continueIndex, 0, 0)
		query.AddKeyVal(common.ZONE_KW, conn.GetAccount().ClientZone)
		query.AddSelect(common.ICAT_COLUMN_COLL_NAME, 1)
		query.AddSelect(common.ICAT_COLUMN_DATA_NAME, 1)

		collection_wildcard_condition := fmt.Sprintf("LIKE '%s'", collection_wildcard_value)
		query.AddCondition(common.ICAT_COLUMN_COLL_NAME, collection_wildcard_condition)

		dataobject_wildcard_condition := fmt.Sprintf("LIKE '%s'", dataobject_wildcard_value)
		query.AddCondition(common.ICAT_COLUMN_DATA_NAME, dataobject_wildcard_condition)

		queryResult := message.IRODSMessageQueryResponse{}
		err := conn.Request(query, &queryResult, nil)
		if err != nil {
			return nil, xerrors.Errorf("failed to receive a collection query result message: %w", err)
		}

		err = queryResult.CheckError()
		if err != nil {
			if types.GetIRODSErrorCode(err) == common.CAT_NO_ROWS_FOUND {
				// empty
				break
			}
			return nil, xerrors.Errorf("received collection query error: %w", err)
		}

		if queryResult.RowCount == 0 {
			break
		}

		collectionResult := queryResult.SQLResult[0]
		dataObjectResult := queryResult.SQLResult[1]
		for row := 0; row < queryResult.RowCount; row++ {
			results = append(results, collectionResult.Values[row]+"/"+dataObjectResult.Values[row])
		}

		continueIndex = queryResult.ContinueIndex
		if continueIndex == 0 {
			continueQuery = false
		}
	}

	return &results, nil
}
