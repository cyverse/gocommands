# Syncing Data Between Local and iRODS

The `sync` command keeps datasets synchronized between local and iRODS by transferring only new or modified files. It runs `get`, `put`, `bput`, and `cp` commands automatically to sync data.

## Syntax:
```sh
gocmd sync <source_path> <destination_path>
```

Source and destination paths can be either local or iRODS paths.
Use `i:<path>` to specify an iRODS path. Local paths do not require a prefix.

Syncing between two local directories is not supported.

## Example Usage:

1. **Sync a local directory to iRODS**:
   ```sh
   gocmd sync /local/dir i:/myZone/home/myUser/dir
   ```

2. **Sync a directory from iRODS to local**:
   ```sh
   gocmd sync i:/myZone/home/myUser/dir /local/dir
   ```

3. **Sync two directories in iRODS**:
   ```sh
   gocmd sync i:/myZone/home/myUser/dir1 i:/myZone/home/myUser/dir2
   ```

## Available Flags for `sync`

### Transfer Mode
| Flag | Description |
|------|-------------|
| `--icat` | Transfer data via iCAT |
| `--redirect` | Redirect transfer to the resource server |

### Parallel Transfer
| Flag | Description |
|------|-------------|
| `--single_threaded` | Use a single thread for file transfer |
| `--bulk_upload` | Use bundle upload for file transfer (for local to iRODS sync) |
| `--thread_num` int | Specify the number of threads for bundle upload (default: 5) |

### Bundle Transfer
| Flag | Description |
|------|-------------|
| `--clear` | Clear stale bundle files before upload (only for bundle upload) |
| `--irods_temp` string | Specify an iRODS temporary path for uploading bundle files (for bundle upload) |
| `--local_temp` string | Specify a local temporary path for creating bundle files (for bundle upload) |
| `--max_file_num` int | Specify the maximum number of files per bundle (for bundle upload, default: 50)  |
| `--min_file_num` int | Specify the minimum number of files per bundle (for bundle upload, default: 3) |
| `--max_file_size` string | Specify the maximum file size per bundle (for bundle upload, default: 2GB) |
| `--no_bulk_reg` | Disable bulk registration (for bundle upload) |

### Differential Transfer
| Flag | Description |
|------|-------------|
| `--diff` | Transfer only files with different content (always on) |
| `--no_hash` | Compare files without using a hash (works with `--diff`) |

### Data Verification
| Flag | Description |
|------|-------------|
| `-k` | Require the data server to calculate a checksum (for local to iRODS sync) |
| `-K, --verify_checksum` | Verify the checksum after transfer |

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

## Useful Tips
The `sync` command works in the same way as `get`, `put`, `bput`, and `cp` commands.

- `gocmd sync <local_source> i:<irods_destination>` is equivalent to:
  ```sh
  gocmd put --diff <local_source> <irods_destination>
  ```
- `gocmd sync --bulk_upload <local_source> i:<irods_destination>` is equivalent to:
  ```sh
  gocmd bput --diff <local_source> <irods_destination>
  ```
- `gocmd sync i:<irods_source> <local_destination>` is equivalent to:  
  ```sh
  gocmd get --diff <irods_source> <local_destination>
  ```
- `gocmd sync i:<irods_source> i:<irods_destination>` is equivalent to:  
  ```sh
  gocmd cp --diff <irods_source> <irods_destination>
  ```

