# Create an Empty Data Object in iRODS

The `touch` command allows you to create an empty data object in iRODS or update the modification time of an existing data object. This functionality is similar to the Unix `touch` command.

## Syntax
```sh
gocmd touch [flags] 
```

## Example Usage

1. **Create an empty data object:**
    ```sh
    gocmd touch /myZone/home/myUser/newfile.txt
    ```
    This command creates an empty data object named `newfile.txt` in the specified iRODS path. If the data object already exists, it updates its modification time.

2. **Update the modification time of an existing data object without creating a new one:**
    ```sh
    gocmd touch --no_create /myZone/home/myUser/oldfile.txt
    ```
    This command updates the modification time of the existing data object `oldfile.txt`. If the specified data object does not exist, the command will fail without creating a new one.


## All Available Flags

| Flag                        | Description                                                                 |
|-----------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`       | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`               | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`                | Display help information about available commands and options.              |
| `--log_file string`         | Specify file path for logging output.                                       |
| `--log_level string`        | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--log_terminal`            | Enable logging to terminal.                                                 |
| `-N, --no`                  | No to all questions.                                                        |
| `--no_create`               | Skip creation of the data object.                                           |
| `-q, --quiet`               | Suppress all non-error output messages.                                     |
| `-r, --reference string`    | Use the modification time of the data object given instead of the current time. Cannot be used with -s. |
| `-n, --replica int`         | The replica number of the replica to update. Cannot be used with -R.        |
| `-R, --resource string`     | Target specific iRODS resource server for operations.                       |
| `--seconds-since-epoch int` | Use the modification time given in seconds since epoch instead of the current time. Cannot be used with -r. |
| `-s, --session int`         | Specify session identifier for tracking operations (default: parent process ID). |
| `--timeout int`             | Specify timeout duration in seconds (default 300).                          |
| `-v, --version`             | Display version information.                                                |
| `-Y, --yes`                 | Yes to all questions.                                                       |
