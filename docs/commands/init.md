# Initialize GoCommands Configuration

The `init` command sets up the iRODS Host and access account for use with other GoCommands tools. Once the configuration is set, configuration files are created under the `~/.irods` directory. The configuration is fully compatible with that of iCommands.

## Syntax

```sh
gocmd init [flags]
```

## Example Usage
1. **Configure interactively:**
    ```sh
    gocmd init
    ```

2. **Configure with existing iCommands configuration directory:**
    ```sh
    gocmd init -c /opt/icommands_credential
    ```

3. **Configure with external YAML/JSON files:**
    ```sh
    gocmd init -c myconfig.yaml
    ```

    This command initializes GoCommands using the configuration specified in the myconfig.yaml file. The configuration file can be in YAML or JSON format and should contain the necessary iRODS connection details.

    Using a custom configuration file allows for more flexible and automated initialization, especially in scenarios where you need to interact with multiple iRODS instances or when working in containerized environments. This is particularly useful when you want to initialize GoCommands within a pipeline or script with a pre-populated configuration file.

4. **Configure with specifing the authentication token's time-to-live (TTL):**
    ```sh
    gocmd init --ttl 24
    ```

    This command sets the authentication token's time-to-live to 24 hours for PAM authentication. The TTL determines how long the temporary iRODS authentication token generated during PAM authentication remains valid. This is particularly useful for controlling session duration and enhancing security.
    

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
| `--ttl int`           | Specify the password time-to-live (TTL) in hours for PAM authentication.   |
