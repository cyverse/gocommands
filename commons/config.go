package commons

import (
	irodsclient_types "github.com/cyverse/go-irodsclient/irods/types"
	"golang.org/x/xerrors"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v2"
)

const (
	PortDefault int = 1247

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
	LogLevel                int    `yaml:"irods_log_level,omitempty" envconfig:"IRODS_LOG_LEVEL"`
	Password                string `yaml:"irods_user_password,omitempty" envconfig:"IRODS_USER_PASSWORD"`
	Ticket                  string `yaml:"irods_ticket,omitempty" envconfig:"IRODS_TICKET"`
	AuthenticationScheme    string `yaml:"irods_authentication_scheme,omitempty" envconfig:"IRODS_AUTHENTICATION_SCHEME"`
	ClientServerNegotiation string `yaml:"irods_client_server_negotiation,omitempty" envconfig:"IRODS_CLIENT_SERVER_NEGOTIATION"`
	ClientServerPolicy      string `yaml:"irods_client_server_policy,omitempty" envconfig:"IRODS_CLIENT_SERVER_POLICY"`
	SSLCACertificateFile    string `yaml:"irods_ssl_ca_certificate_file,omitempty" envconfig:"IRODS_SSL_CA_CERTIFICATE_FILE"`
	EncryptionKeySize       int    `yaml:"irods_encryption_key_size,omitempty" envconfig:"IRODS_ENCRYPTION_KEY_SIZE"`
	EncryptionAlgorithm     string `yaml:"irods_encryption_algorithm,omitempty" envconfig:"IRODS_ENCRYPTION_ALGORITHM"`
	EncryptionSaltSize      int    `yaml:"irods_encryption_salt_size,omitempty" envconfig:"IRODS_ENCRYPTION_SALT_SIZE"`
	EncryptionNumHashRounds int    `yaml:"irods_encryption_num_hash_rounds,omitempty" envconfig:"IRODS_ENCRYPTION_NUM_HASH_ROUNDS"`

	NoReplication bool `yaml:"irods_no_replication,omitempty" envconfig:"IRODS_NO_REPLICATION"`
}

func GetDefaultConfig() *Config {
	return &Config{
		Port:                    PortDefault,
		AuthenticationScheme:    AuthenticationSchemeDefault,
		ClientServerNegotiation: "",
		ClientServerPolicy:      ClientServerPolicyDefault,
		SSLCACertificateFile:    "",
		EncryptionKeySize:       EncryptionKeySizeDefault,
		EncryptionAlgorithm:     EncryptionAlgorithmDefault,
		EncryptionSaltSize:      SaltSizeDefault,
		EncryptionNumHashRounds: HashRoundsDefault,

		NoReplication: false,
	}
}

// NewConfigFromYAML creates Config from YAML
func NewConfigFromYAML(yamlBytes []byte) (*Config, error) {
	config := GetDefaultConfig()

	err := yaml.Unmarshal(yamlBytes, config)
	if err != nil {
		return nil, xerrors.Errorf("failed to unmarshal YAML - %v", err)
	}

	return config, nil
}

// NewConfigFromENV creates Config from Environmental variables
func NewConfigFromENV() (*Config, error) {
	config := GetDefaultConfig()

	err := envconfig.Process("", config)
	if err != nil {
		return nil, xerrors.Errorf("failed to read config from environmental variables - %v", err)
	}

	return config, nil
}
