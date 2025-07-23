# Bundle Upload Data to iRODS

The `bput` command allows you to efficiently upload multiple small files by bundling them into tar archives before transfer to iRODS. This is particularly useful for directories containing many small files.

## Syntax
```sh
gocmd bput [flags] <local-files-or-dir>... <dest-collection>
```

> **Note:** If the local destination is not specified, the current working directory is used as the destination.

## Example Usage

1. **Upload a directory and its contents:**
    ```sh
    gocmd bput /local/dir /myZone/home/myUser/
    ```

2. **Upload with progress bars:**
    ```sh
    gocmd bput --progress /local/dir /myZone/home/myUser/
    ```

3. **Force upload:**
    ```sh
    gocmd bput -f /local/dir /myZone/home/myUser/
    ```

    This command overwrites the existing file in iRODS without prompting.

4. **Upload only different or new contents:**
    ```sh
    gocmd bput --diff /local/dir /myZone/home/myUser/
    ```

    This command uploads only files that are different or don't exist in the destination. It compares file sizes and checksums to determine which files need updating.

5. **Upload only different or new contents without calculating hash:**
    ```sh
    gocmd bput --diff --no_hash /local/dir /myZone/home/myUser/
    ```

    This command skips hash calculations and compares only file sizes for faster synchronization.

6. **Upload via iCAT:**
    ```sh
    gocmd bput --icat /local/dir /myZone/home/myUser/
    ```

    This command uses iCAT as a transfer broker, useful when direct access to the resource server is unstable.

7. **Upload with specified transfer threads:**
    ```sh
    gocmd bput --thread_num 15 /local/dir /myZone/home/myUser/
    ```

    This command uses up to 15 threads for data transfer, requiring more CPU power and RAM.

8. **Upload and generate report:** 
    ```sh
    gocmd bput --report report.json /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS and generates a JSON report containing details such as paths, file sizes, checksums, and transfer methods.

9. **Upload with specified maximum file count per bundle:**
    ```sh
    gocmd bput --max_file_num 100 /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles containing up to 100 files each.

10. **Upload with specified mininum file count per bundle:**
    ```sh
    gocmd bput --min_file_num 50 /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles containing at least 50 files each.

11. **Upload with specified maximum bundle file size:**
    ```sh
    gocmd bput --max_file_size 10GB /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles with a maximum size of 10GB each.

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--age int`           | Exclude files older than the specified age in minutes.                      |
| `--clear`             | Remove stale bundle files from temporary directories.                       |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/iychoi/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `--delete`            | Delete extra files in the destination directory.                            |
| `--delete_on_success` | Delete the source file after a successful transfer.                         |
| `--diff`              | Only transfer files that have different content than existing destination files. |
| `--exclude_hidden_files` | Skip files and directories that start with '.'.                          |
| `-h, --help`          | Display help information about available commands and options.              |
| `--icat`              | Use iCAT for file transfers.                                                |
| `--irods_temp string` | iRODS collection path for temporary bundle file uploads.                    |
| `--local_temp string` | Local directory path for temporary bundle file creation (default "/tmp").   |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--max_file_num int`  | Maximum number of files to include in a single bundle (default 50).         |
| `--max_file_size string` | Maximum size limit for a single bundle file (default "2147483648").      |
| `--min_file_num int`  | Minimum number of files to include in a single bundle (default 3).          |
| `--no_bulk_reg`       | Disable bulk registration of bundle files.                                  |
| `--no_hash`           | Use file size and modification time instead of hash for file comparison when using '--diff'. |
| `--no_root`           | Avoid creating the root directory at the destination during operation.      |
| `--progress`          | Show progress bars during transfer.                                         |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `--report string`     | Create a transfer report; specify the path for file output. An empty string or '-' outputs to stdout. |
| `-R, --resource string` | Target specific iRODS resource server for operations.                     |
| `--retry int`         | Set the number of retry attempts.                                           |
| `--retry_interval int` | Set the interval between retry attempts in seconds (default 60).           |
| `-s, --session int`   | Specify session identifier for tracking operations (default 94807).         |
| `--show_path`         | Show full file paths in progress bars.                                      |
| `--single_threaded`   | Force single-threaded file transfer.                                        |
| `--tcp_buffer_size string` | Set the TCP socket buffer size (default "1MB").                        |
| `--thread_num int`    | Set the number of transfer threads (default 5).                             |
| `-K, --verify_checksum` | Calculate and verify checksums to ensure data integrity after transfer.   |
| `-v, --version`       | Display version information.                                                |
