package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	irodsclient_config "github.com/cyverse/go-irodsclient/config"
	constant "github.com/cyverse/gocommands/commons/constant"
	"golang.org/x/xerrors"
)

// GetConfigCatalogURL returns the URL of the catalog file
func GetConfigCatalogURL() string {
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/refs/heads/main/catalog/catalog.json", constant.GoCommandsRepoPackagePath)
}

type ConfigCatalog struct {
	Configs map[string]map[string]interface{} `json:"configs,omitempty" yaml:"configs,omitempty"`
}

// NewConfigCatalog reads Catalog from a JSON file
func NewConfigCatalog() (*ConfigCatalog, error) {
	catalogURL := GetConfigCatalogURL()

	// Create a client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Make the GET request
	resp, err := client.Get(catalogURL)
	if err != nil {
		return nil, xerrors.Errorf("failed to make GET request to %s: %w", catalogURL, err)
	}
	defer resp.Body.Close()

	// Check the status code
	if resp.StatusCode != http.StatusOK {
		return nil, xerrors.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	jsonBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, xerrors.Errorf("failed to read response body: %w", err)
	}

	configCatalog := ConfigCatalog{}

	err = json.Unmarshal(jsonBytes, &configCatalog)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal config catalog: %w", err)
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
