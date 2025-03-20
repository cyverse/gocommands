# List Metadata of Data Objects, Collections, Resources, or Users in iRODS

To list metadata of Data Objects, Collections, Resources, or Users in iRODS using GoCommands, use the `lsmeta` command.
## Syntax
```sh
gocmd lsmeta [flags] <irods-object>...
```

### iRODS Objects and Flags

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| Data Object or Collection | `-P` | List metadata of data objects or collections |
| Resource | `-R` | List metadata of resources |
| User | `-U` | List metadata of users |

## Example Usage

1. **List metadata of a data object:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/file.txt
    ```

2. **List metadata of multiple data objects:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt
    ```

3. **List metadata of a collection:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/dir
    ```

4. **List metadata of a resource:**
    ```sh
    gocmd lsmeta -R myResc
    ```

5. **List metadata of a user:**
    ```sh
    gocmd lsmeta -U myUser
    ```

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Set config file or directory (default "/home/iychoi/.irods").               |
| `-d, --debug`                       | Enable debug mode.                                                          |
| `-h, --help`                        | Print help.                                                                 |
| `--log_level string`                | Set log level.                                                              |
| `-l, --long`                        | Display results in long format with additional details.                     |
| `-P, --path`                        | Specify that the target is a data object or collection path.                |
| `-q, --quiet`                       | Suppress usual output messages.                                             |
| `-R, --resource`                    | Specify that the target is a resource.                                      |
| `--reverse_sort`                    | Sort results in reverse order.                                              |
| `-s, --session int`                 | Set session ID (default 256579).                                            |
| `-S, --sort string`                 | Sort results by: name, size, time, or ext (default "name").                 |
| `-U, --user`                        | Specify that the target is a user.                                          |
| `-v, --version`                     | Print version.                                                              |
| `-L, --verylong`                    | Display results in very long format with comprehensive information.         |
