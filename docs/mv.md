# Move (Rename) Data Objects and Collections in iRODS

The `mv` command allows you to move or rename data objects and collections in iRODS. This command is similar to the Unix `mv` command but is adapted for use with iRODS.

## Syntax
```sh
gocmd mv <data-object-or-collection>... <target-data-object-or-collection> [flags]
```

## Example Usage

1. **Rename a data object:**
    ```sh
    gocmd mv /myZone/home/myUser/oldfile.txt /myZone/home/myUser/newfile.txt
    ```

2. **Move a data object to a different collection:**
    ```sh
    gocmd mv /myZone/home/myUser/file.txt /myZone/home/myUser/subcollection/
    ```

3. **Rename a collection:**
    ```sh
    gocmd mv /myZone/home/myUser/oldcollection /myZone/home/myUser/newcollection
    ```

4. **Move multiple data objects:**
    ```sh
    gocmd mv /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt /myZone/home/myUser/targetcollection/
    ```

5. **Move multiple data objects with wildcard:**
    ```sh
    gocmd mv -w /myZone/home/myUser/*.txt /myZone/home/myUser/targetcollection/
    ```

    This command will move all files with the `.txt` extension from the /myZone/home/myUser/ collection to the targetcollection. The asterisk (*) wildcard matches any number of characters in the filename.

    You can use more specific wildcard patterns for precise file selection, such as file*.txt to move all text files starting with "file".

## Important Notes

1. When moving data objects or collections, ensure you have the necessary permissions in both the source and target locations.

2. Moving large collections or numerous data objects may take considerable time, depending on the size and number of files involved.

3. If you're moving data between different storage resources, the physical files will be transferred. This operation might be time-consuming for large data sets.

4. Always double-check your command before executing, especially when moving or renaming important data.

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`                         | Display help information about available commands and options.             |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-R, --resource string`              | Target specific iRODS resource server for operations.                     |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 42938).        |
| `-v, --version`                      | Display version information.                                                |
| `-w, --wildcard`                     | Enable wildcard expansion to search for source files.                      |
