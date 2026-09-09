package subcmd

import (
	"testing"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

func TestBunGetDataTypeAutoDetectsCompoundExtensions(t *testing.T) {
	bun := &BunCommand{}
	testCases := []struct {
		path string
		want irodsclient_types.DataType
	}{
		{path: "/collection/archive.tar", want: irodsclient_types.TAR_FILE_DT},
		{path: "/collection/archive.tar.gz", want: irodsclient_types.GZIP_TAR_DT},
		{path: "/collection/archive.TAR.BZ2", want: irodsclient_types.BZIP2_TAR_DT},
		{path: "/collection/archive.zip", want: irodsclient_types.ZIP_FILE_DT},
	}

	for _, testCase := range testCases {
		t.Run(testCase.path, func(t *testing.T) {
			got, err := bun.getDataType(testCase.path, "")
			if err != nil {
				t.Fatalf("getDataType() error = %v", err)
			}
			if got != testCase.want {
				t.Errorf("getDataType() = %q, want %q", got, testCase.want)
			}
		})
	}
}
