# Initialize GoCommands Configuration

The `init` command sets up the iRODS Host and access account for use with other GoCommands tools. Once the configuration is set, configuration files are created under the `~/.irods` directory. The configuration is fully compatible with that of iCommands.

## Syntax

```sh
gocmd init [flags]
```

## Example Usage
1. Configure interactively:
    ```sh
    gocmd init
    ```

2. Configure with existing iCommands configuration directory:
    ```sh
    gocmd init -c /opt/icommands_credential
    ```

3. Configure with external YAML/JSON files:
    ```sh
    gocmd init -c myconfig.yaml
    ```

This command will prompt for access information to the iRODS Host if not populated in the provided configuration.


## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`          | Display help information about available commands and options.             |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`         | Suppress all non-error output messages.                                    |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).        |
| `-v, --version`       | Display version information.                                               |
| `--ttl int`           | Set the password time-to-live in seconds.                                  |
