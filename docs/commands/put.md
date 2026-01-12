# Upload Data to iRODS

The `put` command allows you to upload data objects (files) and collections (directories) from your local system to iRODS. It is similar to the `cp` command in Unix but operates between your local file system and iRODS.

## Syntax
```sh
gocmd put [flags] <local-files-or-dir>... <dest-data-object-or-collection>
```

> **Note:** If the local destination is not specified, the current working directory is used as the destination.

## Example Usage

1. **Upload a single file:**
    ```sh
    gocmd put /local/path/file.txt /myZone/home/myUser/
    ```

    This command uploads the file `/local/path/file.txt` to `/myZone/home/myUser/`, creating `/myZone/home/myUser/file.txt` in iRODS.

2. **Upload a directory and its contents:**
    ```sh
    gocmd put /local/dir /myZone/home/myUser/
    ```

    This command uploads the contents of the directory `/local/dir` to `/myZone/home/myUser/dir` in iRODS. The uploaded files and subdirectories will be placed within the `/myZone/home/myUser/dir` folder.

3. **Upload a data object to a specific iRODS path:**
    ```sh
    gocmd put /local/path/file.txt /myZone/home/myUser/specific_file.txt
    ```

4. **Upload with progress bars:**
    ```sh
    gocmd put --progress /local/path/largefile.dat /myZone/home/myUser/
    ```

5. **Force upload:**
    ```sh
    gocmd put -f /local/path/largefile.dat /myZone/home/myUser/
    ```

    This command overwrites the existing file in iRODS without prompting.

6. **Upload and verify checksum:**
    ```sh
    gocmd put -K /local/path/important_data.txt /myZone/home/myUser/
    ```

    This command uploads the file and verifies its integrity by calculating a checksum during transfer.

7. **Upload only different or new contents:**
    ```sh
    gocmd put --diff /local/dir /myZone/home/myUser/
    ```

    This command uploads only files that are different or don't exist in the destination. It compares file sizes and checksums to determine which files need updating.

8. **Upload only different or new contents without calculating hash:**
    ```sh
    gocmd put --diff --no_hash /local/dir /myZone/home/myUser/
    ```

    This command skips hash calculations and compares only file sizes for faster synchronization.

9. **Upload via iCAT:**
    ```sh
    gocmd put --icat /local/dir /myZone/home/myUser/
    ```

    This command uses iCAT as a transfer broker, useful when direct access to the resource server is unstable.

10. **Upload with specified transfer threads:**
    ```sh
    gocmd put --thread_num 15 /local/dir /myZone/home/myUser/
    ```

    This command uses up to 15 threads for data transfer, requiring more CPU power and RAM.

11. **Upload and generate report:** 
    ```sh
    gocmd put --report report.json /local/dir /myZone/home/myUser/
    ```

    This command uploads files from the local directory to iRODS and generates a JSON report containing details such as paths, file sizes, checksums, and transfer methods.

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--age int`           | Exclude files older than the specified age in minutes.                      |
| `-k, --checksum`      | Generate checksum on the server side after data upload.                     |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `--delete`            | Delete extra files in the destination directory.                            |
| `--delete_on_success` | Delete the source file after a successful transfer.                         |
| `--diff`              | Only transfer files that have different content than existing destination files. |
| `--encrypt`           | Enable file encryption.                                                     |
| `--encrypt_key string` | Specify the encryption key for 'winscp' and 'pgp' mode.                    |
| `--encrypt_mode string` | Specify encryption mode ('winscp', 'pgp', or 'ssh') (default "ssh").      |
| `--encrypt_pub_key string` | Provide the encryption public (or private) key for 'ssh' mode (default "/home/myUser/.ssh/id_rsa.pub"). |
| `--encrypt_temp string` | Set a temporary directory path for file encryption (default "/tmp").      |
| `--exclude_hidden_files` | Skip files and directories that start with '.'.                          |
| `-f, --force`         | Run operation forcefully, bypassing safety checks.                          |
| `-h, --help`          | Display help information about available commands and options.              |
| `--icat`              | Use iCAT for file transfers.                                                |
| `--ignore_meta`       | Ignore encryption config via metadata.                                      |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--no_encrypt`        | Disable file encryption forcefully.                                         |
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
| `-T, --ticket string` | Specify the name of the ticket.                                             |
| `-K, --verify_checksum` | Calculate and verify checksums to ensure data integrity after transfer.   |
| `-v, --version`       | Display version information.                                                |
