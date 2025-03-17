# Copy Data Objects and Collections in iRODS

The `cp` command allows you to copy data objects and collections in iRODS. This command is similar to the Unix `cp` command but is adapted for use with iRODS.

## Syntax
```sh
gocmd cp [flags] <source-data-object-or-collection>... <target-data-object-or-collection>
```

## Example Usage

1. **Copy a data object:**
    ```sh
    gocmd cp /myZone/home/myUser/sourcefile.txt /myZone/home/myUser/destfile.txt
    ```

2. **Copy a data object to a different collection:**
    ```sh
    gocmd cp /myZone/home/myUser/file.txt /myZone/home/myUser/subcollection/
    ```

3. **Copy a collection:**
    ```sh
    gocmd cp -r /myZone/home/myUser/sourcecollection /myZone/home/myUser/destcollection
    ```

    This command will copy the entire `sourcecollection` and all its contents (including subdirectories and files) to `destcollection`.

4. **Copy multiple data objects:**
    ```sh
    gocmd cp /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt /myZone/home/myUser/targetcollection/
    ```

5. **Copy a data object forcefully, overwriting existing data object at destination:**
    ```sh
    gocmd cp -f /myZone/home/myUser/sourcefile.txt /myZone/home/myUser/destfile.txt
    ```

    This command copies `sourcefile.txt` to `destfile.txt`, overwriting the destination without prompting for confirmation.


## Important Notes

1. When copying data objects or collections, ensure you have the necessary read permissions in the source location and write permissions in the target location.

2. Use the `-r` flag when copying collections to perform a recursive copy.

3. Copying large collections or numerous data objects may take considerable time, depending on the size and number of files involved.

4. If you're copying data between different storage resources, the physical files will be transferred. This operation might be time-consuming for large data sets.

5. Always double-check your command before executing, especially when copying important data.

6. By default, `cp` will not overwrite existing files. Use the `-f` flag to force overwriting if needed.

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `--age int`                          | Exclude files older than the specified age in minutes.                     |
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `--delete`                           | Delete extra files in the destination directory.                            |
| `--diff`                             | Only transfer files that have different content than existing destination files. |
| `--exclude_hidden_files`             | Skip files and directories that start with '.'.                             |
| `-f, --force`                        | Run operation forcefully, bypassing safety checks.                          |
| `-h, --help`                         | Display help information about available commands and options.             |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `--no_hash`                          | Use file size and modification time instead of hash for file comparison when using '--diff'. |
| `--no_root`                          | Avoid creating the root directory at the destination during operation.    |
| `--progress`                         | Show progress bars during transfer.                                       |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-r, --recursive`                    | Recursively process operations for collections and their contents.        |
| `--report string`                    | Create a transfer report; specify the path for file output. An empty string or '-' outputs to stdout. |
| `-R, --resource string`               | Target specific iRODS resource server for operations.                     |
| `--retry int`                        | Set the number of retry attempts.                                          |
| `--retry_interval int`                | Set the interval between retry attempts in seconds (default 60).          |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 42938).        |
| `--show_path`                        | Show full file paths in progress bars.                                     |
| `-v, --version`                      | Display version information.                                                |
| `-w, --wildcard`                     | Enable wildcard expansion to search for source files.                      |
