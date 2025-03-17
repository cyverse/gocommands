# Sync Data Between Local and iRODS

The `sync` command efficiently synchronizes datasets between local storage and iRODS by transferring only new or modified files. It automatically runs `get`, `put`, `bput`, and `cp` commands as needed to keep data in sync.

## Syntax
```sh
gocmd sync [flags] <source>... <destination>
```

Source and destination paths can be either local or iRODS paths.
Use `i:<path>` to specify an iRODS path. Local paths do not require a prefix.

> **Note:** Syncing between two local directories is not supported.

## Example Usage

1. **Sync local directory to iRODS:**
    ```sh
    gocmd sync /local/dir i:/myZone/home/myUser/dir
    ```

    This is equivalent to:
    ```sh
    gocmd put --diff /local/dir /myZone/home/myUser/dir
    ```

2. **Sync local directory to iRODS without creating a root at destination:**
    ```sh
    gocmd sync --no_root i:/local/dir i:/myZone/home/myUser/dir
    ```

    This will not create a root directory `dir` under `/myZone/home/myUser/dir`. Without the `--no_root` flag, a directory `/myZone/home/myUser/dir/dir` would be created for sync.

3. **Sync local directory to iRODS with progress-bars:**
    ```sh
    gocmd sync --progress i:/local/dir i:/myZone/home/myUser/dir
    ```

    The `--progress` flag enables progress bars during the synchronization process, providing visual feedback on the transfer's status, including the amount of data transferred and remaining files.

3. **Sync local directory containing many small files to iRODS with bundle-transfer:**
    ```sh
    gocmd sync --bulk_upload /local/dir i:/myZone/home/myUser/dir
    ```

    This command efficiently transfers directories with many small files by bundling them into tar archives before upload.

    This is equivalent to:
    ```sh
    gocmd bput --diff /local/dir /myZone/home/myUser/dir
    ```

4. **Sync a directory from iRODS to local:**
    ```sh
    gocmd sync i:/myZone/home/myUser/dir /local/dir
    ```

    This is equivalent to:
    ```sh
    gocmd get --diff /myZone/home/myUser/dir /local/dir
    ```

5. **Sync two directories in iRODS:**
    ```sh
    gocmd sync i:/myZone/home/myUser/dir1 i:/myZone/home/myUser/dir2
    ```

    This is equivalent to:
    ```sh
    gocmd cp --diff /myZone/home/myUser/dir1 /myZone/home/myUser/dir2
    ```

6. **Sync local directory to iRODS without calculating hash:**
    ```sh
    gocmd sync --no_hash /local/dir i:/myZone/home/myUser/dir
    ```

    This command synchronizes a local directory to iRODS using file paths and sizes to detect changes, skipping hash calculations for faster synchronization. It's ideal for large datasets where speed is prioritized over content integrity checks.

7. **Sync local directory to iRODS with transfer via iCAT:**
    ```sh
    gocmd sync --icat /local/dir i:/myZone/home/myUser/dir
    ```

    This command uses iCAT as a transfer broker. It is useful when direct access to the resource server is unstable.

8. **Sync local directory to iRODS with direct transfer via resource server:**
    ```sh
    gocmd sync --redirect /local/dir i:/myZone/home/myUser/dir
    ```

    This command uses direct access to the resource server. It is useful when transferring large files.

9. **Sync local directory to iRODS with specifying transfer threads:**
    ```sh
    gocmd sync --thread_num 15 /local/dir i:/myZone/home/myUser/dir
    ```

    This command uses up to 15 threads for transfer, requiring more CPU power and RAM.

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--age int`           | Exclude files older than the specified age in minutes.                      |
| `--bulk_upload`       | Enable bulk upload for synchronization.                                     |
| `-k, --checksum`      | Generate checksum on the server side after data upload (default true).      |
| `--clear`             | Remove stale bundle files from temporary directories.                       |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `--delete`            | Delete extra files in the destination directory.                             |
| `-h, --help`          | Display help information about available commands and options.              |
| `--icat`              | Use iCAT for file transfers.                                                 |
| `--irods_temp string` | iRODS collection path for temporary bundle file uploads.                     |
| `--local_temp string` | Local directory path for temporary bundle file creation (default "/tmp").    |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--max_file_num int`  | Maximum number of files to include in a single bundle (default 50).          |
| `--max_file_size string` | Maximum size limit for a single bundle file (default "2147483648").      |
| `--min_file_num int`  | Minimum number of files to include in a single bundle (default 3).           |
| `--no_bulk_reg`       | Disable bulk registration of bundle files.                                  |
| `--no_hash`           | Use file size and modification time instead of hash for file comparison when using '--diff'. |
| `--no_root`           | Avoid creating the root directory at the destination during operation.       |
| `--progress`          | Show progress bars during transfer.                                          |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `--redirect`          | Enable transfer redirection to the resource server.                         |
| `-R, --resource string` | Target specific iRODS resource server for operations.                     |
| `--retry int`         | Set the number of retry attempts.                                            |
| `--retry_interval int` | Set the interval between retry attempts in seconds (default 60).            |
| `-s, --session int`   | Specify session identifier for tracking operations (default 94807).         |
| `--show_path`         | Show full file paths in progress bars.                                       |
| `--single_threaded`   | Force single-threaded file transfer.                                         |
| `--tcp_buffer_size string` | Set the TCP socket buffer size (default "1MB").                        |
| `--thread_num int`    | Set the number of transfer threads (default 5).                            |
| `-K, --verify_checksum` | Calculate and verify checksums to ensure data integrity after transfer (default true). |
| `-v, --version`       | Display version information.                                                |
