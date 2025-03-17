# Remove Data Objects in iRODS

The `rm` command allows you to remove data objects (files) in iRODS. This is similar to the Unix `rm` command but operates within the iRODS environment.

## Syntax
```sh
gocmd rm [flags] <data-object>
```

## Example Usage

1. **Remove a single data object:**
    ```sh
    gocmd rm /myZone/home/myUser/file.txt
    ```

2. **Remove a collection and its contents recursively:**
    ```sh
    gocmd rm -r /myZone/home/myUser/parentCollection
    ```

3. **Force remove a collection and its contents recursively:**
    ```sh
    gocmd rm -rf /myZone/home/myUser/parentCollection
    ```

4. **Remove multiple data objects:**
    ```sh
    gocmd rm /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt
    ```

5. **Remove multiple data objects with wildcard:**
    ```sh
    gocmd rm /myZone/home/myUser/file*.txt
    ```

    This command removes all files with names starting with "file" and ending with ".txt" in the specified iRODS collection. Wildcard usage allows for batch removal of multiple files matching the pattern. Use this feature cautiously to avoid unintended deletions.

## Important Notes

1. **Permissions:** Ensure you have the necessary permissions to remove the data objects or collections.

2. **Recursive Removal:** Use the `-r` flag to remove non-empty collections and their contents.

3. **Force Removal:** The `-f` flag bypasses the trash collection and permanently deletes the items.

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
