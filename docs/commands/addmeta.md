# Add Metadata to Data Objects, Collections, Resources, or Users in iRODS

The `addmeta` command allows you to add new metadata to data objects, collections, resources, or users in iRODS.

## Syntax
```sh
gocmd addmeta [flags] <irods-object> <metadata-name> <metadata-value> [metadata-unit]
```

**Note:** The `metadata-unit` parameter is optional.

### iRODS Objects 

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| `data object` or `collection` | `-P` | Add metadata to a data object or collection |
| `resource` | `-R` | Add metadata to a resource |
| `user` | `-U` | Add metadata to a user |

## Example Usage

1. **Add metadata to a data object:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/file.txt meta_name meta_value
    ```

1. **Add metadata to a data object with metadata-unit:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/file.txt meta_name meta_value meta_unit
    ```

3. **Add metadata to a collection:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/dir meta_name meta_value
    ```

4. **Add metadata to a resource:**
    ```sh
    gocmd addmeta -R myResc meta_name meta_value
    ```

5. **Add metadata to a user:**
    ```sh
    gocmd addmeta -U myUser meta_name meta_value
    ```

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Set config file or directory (default "/home/iychoi/.irods").               |
| `-d, --debug`                       | Enable debug mode.                                                          |
| `-h, --help`                        | Print help.                                                                 |
| `--log_level string`                | Set log level.                                                              |
| `-P, --path`                        | Specify that the target is a data object or collection path.                |
| `-q, --quiet`                       | Suppress usual output messages.                                             |
| `-R, --resource`                    | Specify that the target is a resource.                                      |
| `-s, --session int`                 | Set session ID (default 256579).                                            |
| `-U, --user`                        | Specify that the target is a user.                                          |
| `-v, --version`                     | Print version.                                                              |
