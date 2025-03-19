# Configure SFTP Public-key Authentication

GoCommands provides a feature to configure public-key authentication for the Data Store's SFTP service. The `copy-sftp-id` command uploads your local SSH public keys to the Data Store, enabling password-less authentication for the SFTP service.

## Syntax
```sh
gocmd copy-sftp-id [flags]
```

## Example Usage

1. **Copy all SSH public keys from your `~/.ssh` directory:**
    ```sh
    gocmd copy-sftp-id
    ```

    This command automatically detects all SSH public keys for the current local user in the `~/.ssh` directory at local machine and copies them to `/iplant/home/<username>/.ssh/authorized_keys` in the Data Store. This process is similar to standard SSH public-key registration.

2. **Copy the specified SSH public key:**
    ```sh
    gocmd copy-sftp-id -i ~/.ssh/id_rsa.pub
    ```

    This command copies only the SSH public key from the `~/.ssh/id_rsa.pub` file to `/iplant/home/<username>/.ssh/authorized_keys` in the Data Store.

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `--dry_run`                          | Simulate execution without making actual changes.                          |
| `-h, --help`                         | Display help information about available commands and options.             |
| `-i, --identity_file string`         | Specify the path to the SSH private key file.                              |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-R, --resource string`              | Target specific iRODS resource server for operations.                      |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 341474).       |
| `-v, --version`                      | Display version information.                                               |
