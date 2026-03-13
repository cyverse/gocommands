package catalog

import (
	_ "embed"
)

//go:embed catalog.json
var catalogJSON []byte

func GetCatalogJSON() []byte {
	return catalogJSON
}
