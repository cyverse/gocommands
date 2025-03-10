# Running GoCommands using Docker

GoCommands can be run using Docker without installing it directly on your system. This is useful for containerized environments or when you want to avoid installing dependencies.

## Basic Docker Command

The following command will run `ls` to list the user's home directory:

```bash
docker run -ti \
-e IRODS_AUTHENTICATION_SCHEME=native -e IRODS_HOST=<IRODS_HOST> -e IRODS_PORT=<IRODS_PORT> \
-e IRODS_ZONE_NAME=<IRODS_ZONE_NAME> -e IRODS_USER_NAME=<IRODS_USER_NAME> -e IRODS_USER_PASSWORD=<IRODS_USER_PASSWORD> \
cyverse/gocmd ls
```

## Available Environment Variables

GoCommands supports numerous configuration options via environment variables:

### Authentication Variables
```
IRODS_AUTHENTICATION_SCHEME          # Authentication scheme (e.g., "native")
IRODS_AUTHENTICATION_FILE            # Authentication file path
IRODS_USER_NAME                      # iRODS username
IRODS_USER_PASSWORD                  # iRODS password
IRODS_TICKET                         # iRODS ticket
IRODS_PAM_TOKEN                      # PAM token
IRODS_PAM_TTL                        # PAM token time-to-live
```

### Connection Variables
```
IRODS_HOST                           # iRODS host
IRODS_PORT                           # iRODS port
IRODS_ZONE_NAME                      # iRODS zone name
IRODS_CLIENT_ZONE_NAME               # iRODS client zone name
IRODS_CLIENT_USER_NAME               # iRODS client username
IRODS_CLIENT_SERVER_NEGOTIATION      # Client-server negotiation
IRODS_CLIENT_SERVER_POLICY           # Client-server policy
```

### Path and Resource Variables
```
IRODS_DEFAULT_RESOURCE               # Default resource
IRODS_CWD                            # Current working directory in iRODS
IRODS_HOME                           # Home directory in iRODS
```

### Hash and Encryption Variables
```
IRODS_DEFAULT_HASH_SCHEME            # Default hash scheme
IRODS_MATCH_HASH_POLICY              # Match hash policy
IRODS_ENCRYPTION_ALGORITHM           # Encryption algorithm
IRODS_ENCRYPTION_KEY_SIZE            # Encryption key size
IRODS_ENCRYPTION_SALT_SIZE           # Encryption salt size
IRODS_ENCRYPTION_NUM_HASH_ROUNDS     # Number of encryption hash rounds
```

### SSL Variables
```
IRODS_SSL_CA_CERTIFICATE_FILE        # SSL CA certificate file
IRODS_SSL_CA_CERTIFICATE_PATH        # SSL CA certificate path
IRODS_SSL_VERIFY_SERVER              # SSL verify server
IRODS_SSL_CERTIFICATE_CHAIN_FILE     # SSL certificate chain file
IRODS_SSL_CERTIFICATE_KEY_FILE       # SSL certificate key file
IRODS_SSL_DH_PARAMS_FILE             # SSL DH params file
IRODS_SSL_SERVER_NAME                # SSL server name
```

### Debugging Variables
```
IRODS_DEBUG                          # Enable debug mode
IRODS_LOG_LEVEL                      # Log level
```