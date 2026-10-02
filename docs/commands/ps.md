# Display the Processes for iRODS Connections

The `ps` command lists the processes for iRODS connections established on the iRODS server. This command is useful for monitoring and managing active iRODS sessions.

## Syntax
```sh
gocmd ps [flags]
```

## Example Usage

1. **Display all processes:**
    ```sh
    gocmd ps
    ```

    This command displays all processes serving iRODS access to clients. It is useful when you want to monitor activities across the entire iRODS system, giving you a comprehensive view of all current connections.

2. **Display processes only from a specific IP address:**
    ```sh
    gocmd ps --address 10.11.20.21
    ```

    This command filters and displays only the processes initiated from the specified IP address `10.11.20.21`. It is useful when you want to monitor activity from a particular client machine, which can help in troubleshooting or auditing access from specific locations.

3. **Group processes by client program:**
    ```sh
    gocmd ps --groupbyprog
    ```

    This command groups the displayed processes by the client program used to establish the connection (e.g., GoCommands or iCommands). It is helpful for identifying how different tools are interacting with the iRODS server, allowing you to assess the usage patterns of various client applications.

4. **Group processes by user:**
    ```sh
    gocmd ps --groupbyuser 
    ```

    This command groups the displayed processes by the user who initiated them. It's particularly useful for administrators who want to monitor user activity, identify heavy users, or troubleshoot user-specific issues. This grouping can provide insights into user behavior and resource utilization patterns.

5. **Display processes only from a specific zone:**
    ```sh
    gocmd ps --zone myZone
    ```

    This command filters and displays only the processes associated with the specified iRODS zone (in this case, `myZone`). It's particularly useful in federated iRODS environments where multiple zones exist. This allows administrators to focus on activities within a specific zone, facilitating zone-specific monitoring and management tasks.


## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--address string`    | Display processes from the specified address.                               |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `--groupbyprog`       | Group processes by client program.                                          |
| `--groupbyuser`       | Group processes by user.                                                    |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_file string`   | Specify file path for logging output.                                       |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--log_terminal`      | Enable logging to terminal.                                                 |
| `-N, --no`            | No to all questions.                                                        |
| `--output_csv`        | Display results in CSV format.                                              |
| `--output_json`       | Display results in JSON format.                                             |
| `--output_tsv`        | Display results in TSV format.                                              |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default: parent process ID). |
| `--timeout int`       | Specify timeout duration in seconds (default 300).                          |
| `-v, --version`       | Display version information.                                                |
| `-Y, --yes`           | Yes to all questions.                                                       |
| `--zone string`       | Display processes from the specified zone.                                  |
