# Remove a Collection in iRODS

The `rmdir` command allows you to remove an empty collection (directory) in iRODS. This is similar to the Unix `rmdir` command but operates within the iRODS environment.

## Syntax
```sh
gocmd rmdir [flags] <collection>
```

## Example Usage

1. **Remove an empty collection:**
    ```sh
    gocmd rmdir /myZone/home/myUser/emptyCollection
    ```

2. **Remove a collection and its contents recursively:**
    ```sh
    gocmd rmdir -r /myZone/home/myUser/parentCollection
    ```

    This is equivalent to:
    ```sh
    gocmd rm -r /myZone/home/myUser/parentCollection
    ```

3. **Remove a collection and its contents recursively forcefully:**
    ```sh
    gocmd rmdir -rf /myZone/home/myUser/parentCollection
    ```

## Important Notes

1. **Permissions:** Ensure you have the necessary permissions to remove the collection. Use the following command to check permissions:
    ```sh
    gocmd ls -A
    ```

2. **Non-empty Collections:** The `rmdir` command will fail if the collection is not empty. To remove non-empty collections, use the `-r` flag.

3. **Relative and Absolute Paths:** You can specify either an absolute path or a relative path based on your current working collection.

4. **Error Handling:** If you encounter an error while creating a collection, verify that:
   - The specified path is correct.
   - You have sufficient permissions to remove the collection.
   - The collection is empty (unless using `-r`).

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-f, --force`         | Run operation forcefully, bypassing safety checks.                          |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-r, --recursive`     | Recursively process operations for collections and their contents.          |
| `-R, --resource string` | Target specific iRODS resource server for operations.                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 94807).         |
| `-v, --version`       | Display version information.                                                |
| `-w, --wildcard`      | Enable wildcard expansion to search for source files.                       |

