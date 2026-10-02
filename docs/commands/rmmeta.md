# Remove Metadata from Data Objects, Collections, Resources, or Users in iRODS

The `rmmeta` command allows you to remove metadata from data objects, collections, resources, or users in iRODS.

## Syntax
```sh
gocmd rmmeta [flags] <irods-object> <metadata-ID-or-name>
```

**Note:** The `metadata-ID` is a numeric identifier for the metadata. It can be obtained from the output of the `lsmeta` command.

### iRODS Objects 

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| `data object` or `collection` | `-P` | Remove metadata from a data object or collection |
| `resource` | `-R` | Remove metadata from a resource |
| `user` | `-U` | Remove metadata from a user |

## Example Usage

1. **Remove metadata from a data object by name:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/file.txt meta_name
    ```

2. **Remove metadata from a data object by ID:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/file.txt 979206950
    ```

3. **Remove metadata from a collection:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/dir meta_name
    ```

4. **Remove metadata from a resource:**
    ```sh
    gocmd rmmeta -R myResc meta_name
    ```

5. **Remove metadata from a user:**
    ```sh
    gocmd rmmeta -U myUser meta_name
    ```

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Set config file or directory (default "/home/myUser/.irods").               |
| `-d, --debug`         | Enable debug mode.                                                          |
| `-h, --help`          | Print help.                                                                 |
| `--id`                | Specify metadata ID instead of AVU.                                         |
| `--log_file string`   | Specify file path for logging output.                                       |
| `--log_level string`  | Set log level.                                                              |
| `--log_terminal`      | Enable logging to terminal.                                                 |
| `-N, --no`            | No to all questions.                                                        |
| `-P, --path`          | Specify that the target is a data object or collection path.                |
| `-q, --quiet`         | Suppress usual output messages.                                             |
| `-R, --resource`      | Specify that the target is a resource.                                      |
| `-s, --session int`   | Set session ID (default: parent process ID).                                |
| `--timeout int`       | Specify timeout duration in seconds (default 300).                          |
| `-U, --user`          | Specify that the target is a user.                                          |
| `-v, --version`       | Print version.                                                              |
| `-Y, --yes`           | Yes to all questions.                                                       |
