package subcmd

import (
	"testing"
	"time"

	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"github.com/cyverse/gocommands/commons/format"
)

func TestIndexAccessesByPathPreservesOrder(t *testing.T) {
	first := &irodsclient_types.IRODSAccess{Path: "/zone/a", UserName: "first"}
	other := &irodsclient_types.IRODSAccess{Path: "/zone/b", UserName: "other"}
	second := &irodsclient_types.IRODSAccess{Path: "/zone/a", UserName: "second"}

	indexed := indexAccessesByPath([]*irodsclient_types.IRODSAccess{first, other, second})
	accessesForA := indexed["/zone/a"]
	if len(accessesForA) != 2 || accessesForA[0] != first || accessesForA[1] != second {
		t.Errorf("accesses for /zone/a = %#v, want [%#v %#v]", accessesForA, first, second)
	}

	if accessesForMissingPath := indexed["/zone/missing"]; len(accessesForMissingPath) != 0 {
		t.Errorf("accesses for missing path = %#v, want none", accessesForMissingPath)
	}
}

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
