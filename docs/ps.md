# Display the Processes for iRODS Connections

The `ps` command lists the processes for iRODS connections established on the iRODS server. This command is useful for monitoring and managing active iRODS sessions.

## Syntax
```sh
gocmd ps [flags]
```

## Example Usage

1. Display all processes:
    ```sh
    gocmd ps
    ```

2. Display processes only from a specific IP address:
    ```sh
    gocmd ps --address 10.11.20.21
    ```

3. Group processes by client program:
    ```sh
    gocmd ps --groupbyprog
    ```

4. Group processes by user:
    ```sh
    gocmd ps --groupbyuser
    ```

5. Display processes only from a specific zone:
    ```sh
    gocmd ps --zone myZone
    ```

## Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--address string`    | Display processes from the specified IP address.                            |
| `--groupbyprog`       | Group processes by client program.                                          |
| `--groupbyuser`       | Group processes by user.                                                    |
| `--zone string`       | Display processes from the specified zone.                                  |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |
