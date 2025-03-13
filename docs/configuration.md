# Configuring GoCommands for iRODS

## Using the `init` command

1. Run the following command to configure GoCommands. Enter your iRODS account credentials when prompted. This will create the configuration file under `~/.irods`:
   ```
   gocmd init
   ```
2. To verify the current environment, use:
   ```
   gocmd env
   ```
   This will display the current configurations.
3. Execute GoCommands for your task:
   ```
   gocmd ls
   ```

## Using iCommands Configuration

GoCommands is compatible with iCommands' configuration files. It can automatically detect and use the existing iCommands configuration files located in `~/.irods`. Additionally, GoCommands creates its own configuration files in this directory, allowing users to work with both iCommands and GoCommands interchangeably.

## Using an External Configuration File (YAML or JSON)

GoCommands can read configurations from YAML or JSON files.

### Using an External YAML Configuration File without `init`
1. Create a file named `config.yaml` using your preferred text editor.
2. Add the following content:
   ```
   irods_host: "data.cyverse.org"
   irods_port: 1247
   irods_user_name: "your username"
   irods_zone_name: "iplant"
   irods_user_password: "your password"
   ```
3. To use this configuration file, provide its path with the `-c` flag when running GoCommands:
   ```
   gocmd -c config.yaml env
   ```
4. Execute GoCommands for your task:
   ```
   gocmd -c config.yaml ls
   ```
5. You can omit sensitive fields like `irods_user_password`, and GoCommands will prompt you for the missing values during runtime.


### Creating Configuration from an External File
To configure GoCommands using an external file:
```
gocmd -c config.yaml init
```

## Using Environmental Variables

GoCommands can read configuration directly from environmental variables, which take precedence over other configuration sources.

### Setting Environmental Variables
1. Export the required variables in your terminal:
   ```
   export IRODS_HOST="data.cyverse.org"
   export IRODS_PORT=1247
   export IRODS_USER_NAME="your username"
   export IRODS_ZONE_NAME="iplant"
   export IRODS_USER_PASSWORD="your password"
   ```
2. Run GoCommands to verify the environment settings:
   ```
   gocmd env
   ```
3. Execute GoCommands for your task:
   ```
   gocmd ls
   ```
4. Similar to YAML/JSON configurations, you can omit sensitive fields like `IRODS_USER_PASSWORD`, and GoCommands will prompt for missing values during runtime.


### Creating Configuration from Environmental Variables

To configure GoCommands using environmental variables:
```
gocmd init
```

GoCommands will prompt you to input only the missing fields.


## Full List of Supported Configuration Fields

Below is a comprehensive list of supported fields, along with their corresponding names in JSON, YAML, and environmental variables:

| Field Name                     | JSON/YAML Key                     | Environmental Variable              | Default Value                    |
|--------------------------------|------------------------------------|-------------------------------------|---------------------------------|
| Authentication Scheme           | `irods_authentication_scheme`     | `IRODS_AUTHENTICATION_SCHEME`       | native                           |
| Authentication File             | `irods_authentication_file`       | `IRODS_AUTHENTICATION_FILE`         | ~/irods/.irodsA                 |
| ClientServerNegotiation        | `irods_client_server_negotiation` | `IRODS_CLIENT_SERVER_NEGOTIATION`   | off                              |
| ClientServerPolicy             | `irods_client_server_policy`       | `IRODS_CLIENT_SERVER_POLICY`        | CS_NEG_REFUSE                    |
| Host                           | `irods_host`                      | `IRODS_HOST`                        |                                 |
| Port                           | `irods_port`                      | `IRODS_PORT`                        | 1247                            |
| ZoneName                       | `irods_zone_name`                 | `IRODS_ZONE_NAME`                   |                                 |
| ClientZoneName                 | `irods_client_zone_name`          | `IRODS_CLIENT_ZONE_NAME`            |                                 |
| Username                       | `irods_user_name`                 | `IRODS_USER_NAME`                   |                                 |
| ClientUsername                 | `irods_client_user_name`          | `IRODS_CLIENT_USER_NAME`            |                                 |
| DefaultResource                | `irods_default_resource`          | `IRODS_DEFAULT_RESOURCE`            |                                 |
| CurrentWorkingDir              | `irods_cwd`                       | `IRODS_CWD`                         |                                 |
| Home                           | `irods_home`                      | `IRODS_HOME`                        |                                 |
| DefaultHashScheme              | `irods_default_hash_scheme`       | `IRODS_DEFAULT_HASH_SCHEME`         | SHA256                           |
| MatchHashPolicy                | `irods_match_hash_policy`         | `IRODS_MATCH_HASH_POLICY`           |                                 |
| Debug                          | `irods_debug`                     | `IRODS_DEBUG`                       |                                 |
| LogLevel                       | `irods_log_level`                 | `IRODS_LOG_LEVEL`                   | 0                               |
| EncryptionAlgorithm            | `irods_encryption_algorithm`      | `IRODS_ENCRYPTION_ALGORITHM`        | AES-256-CBC                      |
| EncryptionKeySize              | `irods_encryption_key_size`       | `IRODS_ENCRYPTION_KEY_SIZE`         | 32                              |
| EncryptionSaltSize             | `irods_encryption_salt_size`      | `IRODS_ENCRYPTION_SALT_SIZE`        | 8                               |
| EncryptionNumHashRounds        | `irods_encryption_num_hash_rounds`| `IRODS_ENCRYPTION_NUM_HASH_ROUNDS`  | 16                              |
| SSLCACertificateFile           | `irods_ssl_ca_certificate_file`   | `IRODS_SSL_CA_CERTIFICATE_FILE`     |                                 |
| SSLCACertificatePath           | `irods_ssl_ca_certificate_path`   | `IRODS_SSL_CA_CERTIFICATE_PATH`     |                                 |
| SSLVerifyServer                | `irods_ssl_verify_server`         | `IRODS_SSL_VERIFY_SERVER`           | hostname                         |
| SSLCertificateChainFile        | `irods_ssl_certificate_chain_file`| `IRODS_SSL_CERTIFICATE_CHAIN_FILE`  |                                 |
| SSLCertificateKeyFile          | `irods_ssl_certificate_key_file`  | `IRODS_SSL_CERTIFICATE_KEY_FILE`    |                                 |
| SSLDHParamsFile                | `irods_ssl_dh_params_file`        | `IRODS_SSL_DH_PARAMS_FILE`          |                                 |
| Password                       | `irods_user_password`             | `IRODS_USER_PASSWORD`               |                                 |
| Ticket                         | `irods_ticket`                    | `IRODS_TICKET`                      |                                 |
| PAMToken                       | `irods_pam_token`                 | `IRODS_PAM_TOKEN`                   |                                 |
| PAMTTL                         | `irods_pam_ttl`                   | `IRODS_PAM_TTL`                     |                                 |
| SSLServerName                  | `irods_ssl_server_name`           | `IRODS_SSL_SERVER_NAME`             |                                 |
