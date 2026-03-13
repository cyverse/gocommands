package catalog

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	"github.com/cyverse/gocommands/catalog"
)

type ConfigCatalog struct {
	Configs map[string]map[string]interface{} `json:"configs,omitempty" yaml:"configs,omitempty"`
}

// NewConfigCatalog reads Catalog from a JSON file
func NewConfigCatalog() (*ConfigCatalog, error) {
	configCatalog := ConfigCatalog{}

	catalogJSON := catalog.GetCatalogJSON()
	err := json.Unmarshal(catalogJSON, &configCatalog)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to unmarshal config catalog")
	}

	return &configCatalog, nil
}

func (catalog *ConfigCatalog) GetIRODSConfig(hostname string) (*irodsclient_config.Config, bool) {
	irodsConfig := irodsclient_config.GetDefaultConfig()

	hostConfig, ok := catalog.Configs[hostname]
	if !ok {
		return irodsConfig, false
	}

	// convert struct to map
	jsonConfig, err := json.Marshal(irodsConfig)
	if err != nil {
		return irodsConfig, false
	}

	mapConfig := map[string]interface{}{}
	err = json.Unmarshal(jsonConfig, &mapConfig)
	if err != nil {
		return irodsConfig, false
	}

	for k, v := range hostConfig {
		mapConfig[k] = v
	}

	// convert map to struct
	jsonConfig, err = json.Marshal(mapConfig)
	if err != nil {
		return irodsConfig, false
	}

	err = json.Unmarshal(jsonConfig, irodsConfig)
	if err != nil {
		return irodsConfig, false
	}

	return irodsConfig, true
}
