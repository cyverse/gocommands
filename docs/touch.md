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

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--no_create`         | Skip creation of the data object. |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-R, --resource string` | Target specific iRODS resource server for operations.                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |
