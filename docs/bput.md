# Bundle Uploading Data to iRODS from a Local Machine

The `bput` command allows you to efficiently upload multiple small files as a bundle from a local directory to iRODS. It first groups the files into a `tar` archive, which is then uploaded and extracted on the server as it arrives. This approach optimizes the upload process, particularly for directories containing many small files.

## Syntax:
```sh
gocmd bput <local_source_path> <iRODS_destination_path>
```

## Example Usage:

1. **Upload a directory of small files** to iRODS:
   ```sh
   gocmd bput /local/dir /myZone/home/myUser/
   ```

2. **Upload multiple directories and files** from different local paths:
   ```sh
   gocmd bput /local/dir1 /local/dir2 /local/dir3/file3 /myZone/home/myUser/
   ```

3. **Upload a directory** to the current iRODS working directory (omit the iRODS destination):
   ```sh
   gocmd bput /local/dir
   ```

## Available Flags for `bput`

### Transfer Mode
| Flag | Description |
|------|-------------|
| `--icat` | Transfer data via iCAT |
| `--redirect` | Redirect transfer to the resource server |

### Parallel Transfer
| Flag | Description |
|------|-------------|
| `--single_threaded` | Use a single thread for file transfer |
| `--thread_num` int | Specify the number of threads for bundle upload (default: 5) |

### Bundle Transfer
| Flag | Description |
|------|-------------|
| `--clear` | Clear stale bundle files before upload |
| `--irods_temp` string | Specify an iRODS temporary path for uploading bundle files |
| `--local_temp` string | Specify a local temporary path for creating bundle files |
| `--max_file_num` int | Specify the maximum number of files per bundle (default: 50) |
| `--min_file_num` int | Specify the minimum number of files per bundle (default: 3) |
| `--max_file_size` string | Specify the maximum file size per bundle (default: 2GB) |
| `--no_bulk_reg` | Disable bulk registration |

### Differential Transfer
| Flag | Description |
|------|-------------|
| `--diff` | Transfer only files with different content |
| `--no_hash` | Compare files without using a hash (works with `--diff`) |

### File Management
| Flag | Description |
|------|-------------|
| `--delete` | Remove extra files from the source directory after upload |
| `--delete_on_success` | Delete local files upon successful transfer |
| `--report` string | Generate a transfer report (output to a specified file or `stdout` if empty or `-`) |

### User Interface and Display
| Flag | Description |
|------|-------------|
| `--progress` | Display progress bars during transfer |
| `--show_path` | Display the full path in progress bars (requires `--progress`) |
| `--no_root` | Do not create a root target directory in iRODS |

### Source File Filters
| Flag | Description |
|------|-------------|
| `--exclude_hidden_files` | Exclude hidden files (those starting with `.`) |
| `--age` int | Exclude files older than the specified age (in minutes) |

### Low-Level Transfer Options
| Flag | Description |
|------|-------------|
| `--tcp_buffer_size` string | Specify the TCP socket buffer size (default: `1MB`) |

## Clear Stale Bundle Files After Failure
The `bclean` command can be used to remove bundle files created by `bput` operation. This helps to clean up incomplete or outdated bundles from both the local and iRODS destinations.

```sh
bclean
```
Or, for cleaning specific destinations:
```sh
bclean <iRODS_destination_path>
```
