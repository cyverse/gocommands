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
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |
