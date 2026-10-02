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

3. **Upload only different or new contents:**
    ```sh
    gocmd bput --diff /local/dir /myZone/home/myUser/
    ```

    This command uploads only files that are different or don't exist in the destination. It compares file sizes and checksums to determine which files need updating.

4. **Upload only different or new contents without calculating hash:**
    ```sh
    gocmd bput --diff --no_hash /local/dir /myZone/home/myUser/
    ```

    This command skips hash calculations and compares only file sizes for faster synchronization.

5. **Upload via iCAT:**
    ```sh
    gocmd bput --icat /local/dir /myZone/home/myUser/
    ```

    This command uses iCAT as a transfer broker, useful when direct access to the resource server is unstable.

6. **Upload with specified transfer threads:**
    ```sh
    gocmd bput --thread_num 15 /local/dir /myZone/home/myUser/
    ```

    This command uses up to 15 threads for data transfer, requiring more CPU power and RAM.

7. **Upload and generate report:** 
    ```sh
    gocmd bput --report report.json /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS and generates a JSON report containing details such as paths, file sizes, checksums, and transfer methods.

8. **Upload with specified maximum file count per bundle:**
    ```sh
    gocmd bput --max_file_num 100 /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles containing up to 100 files each.

9. **Upload with specified mininum file count per bundle:**
    ```sh
    gocmd bput --min_file_num 50 /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles containing at least 50 files each.

10. **Upload with specified maximum bundle file size:**
    ```sh
    gocmd bput --max_bundle_size 10GB /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS by creating bundles with a maximum size of 10GB each.

## All Available Flags

| Flag                            | Description                                                                 |
|---------------------------------|-----------------------------------------------------------------------------|
| `--age int`                     | Exclude files older than the specified age in minutes.                      |
| `--clear`                       | Remove stale bundle files from temporary directories.                       |
| `-c, --config string`           | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                   | Enable verbose debug output for troubleshooting.                            |
| `--delete`                      | Delete extra files in the destination directory.                            |
| `--delete_on_success`           | Delete the source file after a successful transfer.                         |
| `--diff`                        | Only transfer files that have different content than existing destination files. |
| `--encrypt`                     | Enable file encryption.                                                     |
| `--encrypt_key string`          | Specify the encryption key for 'winscp' and 'pgp' mode.                     |
| `--encrypt_mode string`         | Specify encryption mode ('winscp', 'pgp', or 'ssh') (default "ssh").        |
| `--encrypt_pub_key string`      | Provide the encryption public (or private) key for 'ssh' mode (default "/home/myUser/.ssh/id_rsa.pub"). |
| `--encrypt_temp string`         | Set a temporary directory path for file encryption (default "/tmp").        |
| `--exclude_hidden_files`        | Skip files and directories that start with '.'.                             |
| `-h, --help`                    | Display help information about available commands and options.              |
| `--icat`                        | Use iCAT for file transfers.                                                |
| `--ignore_meta`                 | Ignore encryption config via metadata.                                      |
| `--irods_temp string`           | iRODS collection path for temporary bundle file uploads.                    |
| `--local_temp string`           | Local directory path for temporary bundle file creation (default "/tmp").   |
| `--log_file string`             | Specify file path for logging output.                                       |
| `--log_level string`            | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--log_terminal`                | Enable logging to terminal.                                                 |
| `--max_bundle_size string`      | Maximum size limit for a single bundle file (default "2147483648").         |
| `--max_file_num int`            | Maximum number of files to include in a single bundle (default 50).         |
| `--min_file_num int`            | Minimum number of files to include in a single bundle (default 3).          |
| `-N, --no`                      | No to all questions.                                                        |
| `--no_bulk_reg`                 | Disable bulk registration of bundle files.                                  |
| `--no_encrypt`                  | Disable file encryption forcefully.                                         |
| `--no_hash`                     | Use file size and modification time instead of hash for file comparison when using '--diff'. |
| `--no_root`                     | Avoid creating the root directory at the destination during operation.      |
| `--progress`                    | Show progress bars during transfer.                                         |
| `-q, --quiet`                   | Suppress all non-error output messages.                                     |
| `--redirect`                    | Connect to resource servers directly for transfer.                          |
| `--report string`               | Create a transfer report; specify the path for file output. An empty string or '-' outputs to stdout. |
| `-R, --resource string`         | Target specific iRODS resource server for operations.                       |
| `--retry int`                   | Set the number of retry attempts (default 3).                               |
| `--retry_interval int`          | Set the interval between retry attempts in seconds (default 5).             |
| `-s, --session int`             | Specify session identifier for tracking operations (default: parent process ID). |
| `--show_path`                   | Show full file paths in progress bars.                                      |
| `--single_threaded`             | Force single-threaded file transfer.                                        |
| `--stop_on_error`               | Stop all transfers immediately when an error occurs.                        |
| `--tcp_recv_buffer_size string` | Set the TCP socket receive buffer size (default "0").                       |
| `--tcp_send_buffer_size string` | Set the TCP socket send buffer size (default "0").                          |
| `--thread_num int`              | Set the total number of transfer threads (default 5).                       |
| `--thread_num_per_file int`     | Set the number of transfer threads for each file (default 5).               |
| `--timeout int`                 | Specify timeout duration in seconds (default 300).                          |
| `-k, --verify_checksum`         | Calculate and verify checksums to ensure data integrity after transfer.     |
| `-v, --version`                 | Display version information.                                                |
| `--webdav`                      | Use WebDAV protocol (HTTP) for transfer.                                    |
| `-Y, --yes`                     | Yes to all questions.                                                       |
