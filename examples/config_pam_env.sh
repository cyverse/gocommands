# An example of gocommands configuration
# use PAM authentication (via password and SSL CA Certificate)
export IRODS_HOST="data.cyverse.org"
export IRODS_PORT=1247
export IRODS_USER_NAME="your username"
export IRODS_ZONE_NAME="iplant"
export IRODS_USER_PASSWORD="your password"
export IRODS_DEFAULT_RESOURCE=""
export IRODS_CLIENT_USER_NAME=""
export IRODS_LOG_LEVEL=5
export IRODS_AUTHENTICATION_SCHEME="pam"
export IRODS_CLIENT_SERVER_NEGOTIATION="request_server_negotiation"
export IRODS_CLIENT_SERVER_POLICY="CS_NEG_REQUIRE"
export IRODS_SSL_CA_CERTIFICATE_FILE="path to ssl ca certificate file"
export IRODS_ENCRYPTION_KEY_SIZE=32
export IRODS_ENCRYPTION_ALGORITHM="AES-256-CBC"
export IRODS_ENCRYPTION_SALT_SIZE=8
export IRODS_ENCRYPTION_NUM_HASH_ROUNDS=16