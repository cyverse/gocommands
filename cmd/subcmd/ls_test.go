package subcmd

import (
	"testing"
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
)

func TestGetDataObjectModifyTimeWithoutReplicas(t *testing.T) {
	ls := &LsCommand{}
	modifiedAt := ls.getDataObjectModifyTime(&irodsclient_types.IRODSDataObject{})
	if !modifiedAt.Equal(time.Time{}) {
		t.Errorf("getDataObjectModifyTime() = %v, want zero time", modifiedAt)
	}
}
