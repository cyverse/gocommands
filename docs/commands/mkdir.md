# Create Collections in iRODS

The `mkdir` command allows you to create new collections (directories) in iRODS. This is similar to the Unix `mkdir` command but operates within the iRODS environment. Collections in iRODS are used to organize data objects (files) hierarchically.

## Syntax
```sh
gocmd mkdir [flags] <new-collection>...
```

## Example Usage

1. **Create a new collection:**
    ```sh
    gocmd mkdir /myZone/home/myUser/newCollection
    ```

2. **Create parent collections if they do not exist:**
    ```sh
    gocmd mkdir -p /myZone/home/myUser/parentCollection/newCollection
    ```
    This command creates the `newCollection` along with its parent collection `parentCollection` if it does not already exist.


## Important Notes

1. **Permissions:** Ensure you have the necessary write permissions in the parent collection where you want to create the new collection. Use the following command to check permissions:
    ```sh
    gocmd ls -A
    ```

2. **Parent Collections:** If the parent collections do not exist and you do not use the `-p` flag, the command will fail.

3. **Relative and Absolute Paths:** You can specify either an absolute path or a relative path based on your current working collection.

4. **Error Handling:** If you encounter an error while creating a collection, verify that:
   - The specified path is correct.
   - You have sufficient permissions to create collections in the target location.


## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-p, --parents`       | Create parent collections if they do not exist.                             |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-R, --resource string` | Target specific iRODS resource server for operations.                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |
