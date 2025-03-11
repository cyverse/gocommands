# Uploading Data to iRODS from a Local Machine

The `put` command allows you to upload data from a local directory to iRODS.

## Syntax:
```sh
gocmd put <local_source_path> <iRODS_destination_path>
```

## Example Usage:

1. **Upload a single file** from a local directory to iRODS:
   ```sh
   gocmd put myData1.obj /myZone/home/myUser/
   ```

2. **Upload multiple files** by specifying multiple local source paths:
   ```sh
   gocmd put myData1.obj myData2.obj /myZone/home/myUser/
   ```

3. **Upload a file** from a local directory to the current iRODS working directory (omit the iRODS destination):
   ```sh
   gocmd put myData1.obj
   ```

## Available Flags for `put`

### Transfer Mode
| Flag | Description |
|------|-------------|
| `--icat` | Transfer data via iCAT |
| `--redirect` | Redirect transfer to the resource server |

### Parallel Transfer
| Flag | Description |
|------|-------------|
| `--single_threaded` | Use a single thread for file transfer |
| `--thread_num` int | Specify the number of threads (default: 5) |

### Differential Transfer
| Flag | Description |
|------|-------------|
| `--diff` | Transfer files with different content |
| `--no_hash` | Compare files without using a hash (works with `--diff`) |

### Data Verification
| Flag | Description |
|------|-------------|
| `-k` | Require the data server to calculate a checksum |
| `-K, --verify_checksum` | Verify the checksum after transfer |

### File Management
| Flag | Description |
|------|-------------|
| `--delete` | Remove extra files from the source directory |
| `--delete_on_success` | Delete the local file upon successful transfer |
| `--report` string | Generate a transfer report (output to a specified file or `stdout` if empty or `-`) |

### User Interface and Display
| Flag | Description |
|------|-------------|
| `--progress` | Show progress bars during transfer |
| `--show_path` | Display the full path in progress bars (requires `--progress`) |
| `-f, --force` | Force the operation without confirmation prompts |
| `--no_root` | Do not create a root target directory in iRODS |

### Source File Filters
| Flag | Description |
|------|-------------|
| `--exclude_hidden_files` | Exclude hidden files (those starting with `.`) |
| `--age` int | Exclude files older than the specified age (in minutes) |

### Encryption Options
| Flag | Description |
|------|-------------|
| `--encrypt` | Enable file encryption (default: true) |
| `--encrypt_key` string | Specify the encryption key for 'winscp' or 'pgp' modes |
| `--encrypt_priv_key` string | Provide the encryption private key for 'ssh' mode (default: `/home/iychoi/.ssh/id_rsa`) |
| `--encrypt_temp` string | Set a temporary directory for file encryption (default: `/tmp`) |

### Low-Level Transfer Options
| Flag | Description |
|------|-------------|
| `--tcp_buffer_size` string | Specify the TCP socket buffer size (default: `1MB`) |
