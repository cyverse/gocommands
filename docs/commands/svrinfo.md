# Display iRODS Server Information

The `svrinfo` command provides information about the iRODS server, including its iRODS versions and zone.

## Syntax
```sh
gocmd svrinfo [flags]
```

## Example Usage
```sh
gocmd svrinfo
```
This command retrieves and displays information about the connected iRODS server.


The output of the `svrinfo` command may look like this:
```sh
+-----------------+-----------+
| Release Version | rods4.2.11|
| API Version     | d         |
| iRODS Zone      | myZone    |
+-----------------+-----------+
```


## Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
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
