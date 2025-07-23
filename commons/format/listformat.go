package format

import "strings"

type ListFormat string

const (
	// format
	ListFormatNormal   ListFormat = ""
	ListFormatLong     ListFormat = "long"
	ListFormatVeryLong ListFormat = "verylong"
)

type ListSortOrder string

const (
	// sort
	ListSortOrderNone ListSortOrder = ""
	ListSortOrderName ListSortOrder = "name"
	ListSortOrderSize ListSortOrder = "size"
	ListSortOrderTime ListSortOrder = "time"
	ListSortOrderExt  ListSortOrder = "ext"
)

// GetListSortOrder returns ListSortOrder from string
func GetListSortOrder(order string) ListSortOrder {
	switch strings.ToLower(order) {
	case string(ListSortOrderName):
		return ListSortOrderName
	case string(ListSortOrderSize):
		return ListSortOrderSize
	case string(ListSortOrderTime):
		return ListSortOrderTime
	case string(ListSortOrderExt):
		return ListSortOrderExt
	default:
		return ListSortOrderName
	}
}
