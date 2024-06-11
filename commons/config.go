package commons

import (
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

const (
	PortDefault int = 1247

	HashSchemeDefault           string = irodsclient_types.HashSchemeDefault
	AuthenticationSchemeDefault string = string(irodsclient_types.AuthSchemeNative)
	ClientServerPolicyDefault   string = string(irodsclient_types.CSNegotiationRequireTCP)
	EncryptionKeySizeDefault    int    = 32
	EncryptionAlgorithmDefault  string = "AES-256-CBC"
	SaltSizeDefault             int    = 8
	HashRoundsDefault           int    = 16
)

type Config struct {
	CurrentWorkingDir       string `yaml:"irods_cwd,omitempty" envconfig:"IRODS_CWD"`
	Host                    string `yaml:"irods_host,omitempty" envconfig:"IRODS_HOST"`
	Port                    int    `yaml:"irods_port,omitempty" envconfig:"IRODS_PORT"`
	Username                string `yaml:"irods_user_name,omitempty" envconfig:"IRODS_USER_NAME"`
	ClientUsername          string `yaml:"irods_client_user_name,omitempty" envconfig:"IRODS_CLIENT_USER_NAME"`
	Zone                    string `yaml:"irods_zone_name,omitempty" envconfig:"IRODS_ZONE_NAME"`
	DefaultResource         string `yaml:"irods_default_resource,omitempty" envconfig:"IRODS_DEFAULT_RESOURCE"`
	DefaultHashScheme       string `yaml:"irods_default_hash_scheme,omitempty" envconfig:"IRODS_DEFAULT_HASH_SCHEME"`
	LogLevel                int    `yaml:"irods_log_level,omitempty" envconfig:"IRODS_LOG_LEVEL"`
	Password                string `yaml:"irods_user_password,omitempty" envconfig:"IRODS_USER_PASSWORD"`
	Ticket                  string `yaml:"irods_ticket,omitempty" envconfig:"IRODS_TICKET"`
	AuthenticationScheme    string `yaml:"irods_authentication_scheme,omitempty" envconfig:"IRODS_AUTHENTICATION_SCHEME"`
	ClientServerNegotiation string `yaml:"irods_client_server_negotiation,omitempty" envconfig:"IRODS_CLIENT_SERVER_NEGOTIATION"`
	ClientServerPolicy      string `yaml:"irods_client_server_policy,omitempty" envconfig:"IRODS_CLIENT_SERVER_POLICY"`
	SSLCACertificateFile    string `yaml:"irods_ssl_ca_certificate_file,omitempty" envconfig:"IRODS_SSL_CA_CERTIFICATE_FILE"`
	SSLCACertificatePath    string `yaml:"irods_ssl_ca_certificate_path,omitempty" envconfig:"IRODS_SSL_CA_CERTIFICATE_PATH"`
	EncryptionKeySize       int    `yaml:"irods_encryption_key_size,omitempty" envconfig:"IRODS_ENCRYPTION_KEY_SIZE"`
	EncryptionAlgorithm     string `yaml:"irods_encryption_algorithm,omitempty" envconfig:"IRODS_ENCRYPTION_ALGORITHM"`
	EncryptionSaltSize      int    `yaml:"irods_encryption_salt_size,omitempty" envconfig:"IRODS_ENCRYPTION_SALT_SIZE"`
	EncryptionNumHashRounds int    `yaml:"irods_encryption_num_hash_rounds,omitempty" envconfig:"IRODS_ENCRYPTION_NUM_HASH_ROUNDS"`
}

func GetDefaultConfig() *Config {
	return &Config{
		Port:                    PortDefault,
		AuthenticationScheme:    AuthenticationSchemeDefault,
		DefaultHashScheme:       HashSchemeDefault,
		ClientServerNegotiation: "",
		ClientServerPolicy:      ClientServerPolicyDefault,
		SSLCACertificateFile:    "",
		SSLCACertificatePath:    "",
		EncryptionKeySize:       EncryptionKeySizeDefault,
		EncryptionAlgorithm:     EncryptionAlgorithmDefault,
		EncryptionSaltSize:      SaltSizeDefault,
		EncryptionNumHashRounds: HashRoundsDefault,
	}
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(config *Config, yamlBytes []byte) (*Config, error) {
	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}

// NewConfigFromENV creates Config from Environmental variables
func NewConfigFromENV() (*Config, error) {
	config := &Config{}
	err := envconfig.Process("", config)
	if err != nil {
		return nil, xerrors.Errorf("failed to read config from environmental variables: %w", err)
	}

	return config, nil
}

// GetDefaultIRODSConfigPath returns default config path
func GetDefaultIRODSConfigPath() string {
	irodsConfigPath, err := ExpandHomeDir("~/.irods")
	if err != nil {
		return ""
	}

	return irodsConfigPath
}

type ConfigTypeIn struct {
	Host     string `yaml:"irods_host,omitempty"`
	Port     int    `yaml:"irods_port,omitempty"`
	Zone     string `yaml:"irods_zone_name,omitempty"`
	Username string `yaml:"irods_user_name,omitempty"`
	Password string `yaml:"irods_user_password,omitempty"`
}

// NewConfigTypeInFromYAML creates ConfigTypeIn from YAML
func NewConfigTypeInFromYAML(yamlBytes []byte) (*ConfigTypeIn, error) {
	config := &ConfigTypeIn{}

	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML: %w", err)
	}

	return config, nil
}

func (config *ConfigTypeIn) ToYAML() ([]byte, error) {
	yamlBytes, err := yaml.Marshal(config)
	if err != nil {
		return nil, xerrors.Errorf("failed to marshal to YAML: %w", err)
	}
	return yamlBytes, nil
}
