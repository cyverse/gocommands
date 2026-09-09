package subcmd

import (
	"testing"
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons/format"
)

func TestGetDataObjectModifyTimeWithoutReplicas(t *testing.T) {
	ls := &LsCommand{}
	modifiedAt := ls.getDataObjectModifyTime(&irodsclient_types.IRODSDataObject{})
	if !modifiedAt.Equal(time.Time{}) {
		t.Errorf("getDataObjectModifyTime() = %v, want zero time", modifiedAt)
	}
}

func TestReverseSortFallbacks(t *testing.T) {
	collections := []*irodsclient_types.IRODSCollection{{Name: "b"}, {Name: "a"}}
	if !(&LsCommand{}).getCollectionSortFunction(collections, format.ListSortOrderSize, true)(0, 1) {
		t.Error("reverse collection fallback did not sort names descending")
	}

	metas := []*irodsclient_types.IRODSMeta{{AVUID: 2}, {AVUID: 1}}
	if !(&LsMetaCommand{}).getMetaSortFunction(metas, format.ListSortOrderSize, true)(0, 1) {
		t.Error("reverse metadata fallback did not sort AVUIDs descending")
	}

	tickets := []*irodsclient_types.IRODSTicket{{Name: "b"}, {Name: "a"}}
	if !(&LsTicketCommand{}).getTicketSortFunction(tickets, format.ListSortOrderSize, true)(0, 1) {
		t.Error("reverse ticket fallback did not sort names descending")
	}
}
