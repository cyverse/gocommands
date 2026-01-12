# Download Data From iRODS

The `get` command allows you to download data objects (files) and collections (directories) from iRODS to your local system. It is similar to the `cp` command in Unix but operates between iRODS and your local file system.

## Syntax
```sh
gocmd get [flags] <data-object-or-collection>... <dest-local-file-or-dir> 
```

> **Note:** If the local path is not specified, the current working directory is used as the destination.

## Example Usage

1. **Download a single data object to the current working directory:**
    ```sh
    gocmd get /myZone/home/myUser/file.txt
    ```

2. **Download a collection and its contents to the current working directory:**
    ```sh
    gocmd get /local/dir
    ```

3. **Download a data object to a specific local path:**
    ```sh
    gocmd get /myZone/home/myUser/file.txt /local/path/file_new_name.txt
    ```

    This command downloads the data object `/myZone/home/myUser/file.txt` and saves it as `/local/path/file_new_name.txt`.

4. **Download with progress bars:**
    ```sh
    gocmd get --progress /myZone/home/myUser/largefile.dat /local/dir/
    ```

5. **Force download:**
    ```sh
    gocmd get -f /myZone/home/myUser/largefile.dat
    ```

    This command overwrites the local file without prompting if it already exists.

6. **Download and verify checksum:**
    ```sh
    gocmd get -K /myZone/home/myUser/important_data.txt
    ```

    This command downloads the data object and verifies its integrity by calculating the checksum after download and comparing it with the original in iRODS. This ensures data consistency and detects any corruption during transfer.

7. **Download only different or new contents:**
    ```sh
    gocmd get --diff /myZone/home/myUser/dir /local/dir
    ```

    This command downloads the source collection to the local directory, transferring only files that are different or don't exist locally. It compares file sizes and checksums to determine which files need updating, making the transfer more efficient by skipping unchanged files.

8. **Download only different or new contents without calculating hash:**
    ```sh
    gocmd get --diff --no_hash /myZone/home/myUser/dir /local/dir
    ```

    This command downloads the source collection to the local directory, transferring only files that are different or don't exist locally. It skips hash calculations and compares only file sizes for faster synchronization

9. **Download via iCAT:**
    ```sh
    gocmd get --icat /myZone/home/myUser/dir /local/dir
    ```

    This command uses iCAT as a transfer broker, which is useful when direct access to the resource server is unstable. It ensures reliable data transfer by routing through the iCAT server

10. **Download with specified transfer threads:**
    ```sh
    gocmd get --thread_num 15 /myZone/home/myUser/dir /local/dir
    ```

    This command uses up to 15 threads for data transfer, requiring more CPU power and RAM.

11. **Download with wildcard:** 
    ```sh
    gocmd get -w /myZone/home/myUser/dir/file*.txt /local/dir
    ```

    This command downloads all files matching the pattern "file*.txt" from the specified iRODS collection to the local directory.

12. **Download and generate report:** 
    ```sh
    gocmd get --report report.json /myZone/home/myUser/dir /local/dir
    ```

    This command downloads the source collection to the local directory and generates a JSON report file containing transfer details such as paths, file sizes, checksums, and transfer methods.


## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--age int`           | Exclude files older than the specified age in minutes.                      |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `--decrypt`           | Enable file decryption (default true).                                      |
| `--decrypt_key string`| Specify the decryption key for 'winscp' or 'pgp' modes.                     |
| `--decrypt_priv_key string` | Provide the decryption private key for 'ssh' mode (default "/home/myUser/.ssh/id_rsa"). |
| `--decrypt_temp string` | Set a temporary directory for file decryption (default "/tmp").           |
| `--delete`            | Delete extra files in the destination directory.                            |
| `--delete_on_success` | Delete the source file after a successful transfer.                         |
| `--diff`              | Only transfer files that have different content than existing destination files. |
| `--exclude_hidden_files` | Skip files and directories that start with '.'.                          |
| `-f, --force`         | Run operation forcefully, bypassing safety checks.                          |
| `-h, --help`          | Display help information about available commands and options.              |
| `--icat`              | Use iCAT for file transfers.                                                |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `--no_decrypt`        | Disable file decryption forcefully.                                         |
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
| `-w, --wildcard`      | Enable wildcard expansion to search for source files.                       |
