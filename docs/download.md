# Downloading Data from iRODS to a Local Machine

The `get` command allows you to retrieve data from iRODS and download it to a local directory.

## Syntax:
```sh
gocmd get <iRODS_source_path> <local_destination>
```

## Example Usage:

1. **Download a single file** to the current local directory:
   ```sh
   gocmd get /myZone/home/myUser/myData1.obj .
   ```

2. **Download multiple files** by specifying multiple iRODS source paths:
   ```sh
   gocmd get /myZone/home/myUser/myData1.obj /myZone/home/myUser/myData2.obj .
   ```

3. **Download a file** to the current working directory (omit the local destination):
   ```sh
   gocmd get /myZone/home/myUser/myData1.obj
   ```

## Available Flags for `get`

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
| `-K, --verify_checksum` | Verify the checksum after transfer |

### File Management
| Flag | Description |
|------|-------------|
| `--delete` | Remove extra files from the destination directory |
| `--delete_on_success` | Delete the source file upon successful transfer |
| `--report` string | Generate a transfer report (output to a specified file or `stdout` if empty or `-`) |

### User Interface and Display
| Flag | Description |
|------|-------------|
| `--progress` | Show progress bars during transfer |
| `--show_path` | Display the full path in progress bars (requires `--progress`) |
| `-f, --force` | Force the operation without confirmation prompts |
| `--no_root` | Do not create a root target directory |

### Source File Filters
| Flag | Description |
|------|-------------|
| `--exclude_hidden_files` | Exclude hidden files (those starting with `.`) |
| `-w, --wildcard` | Enable wildcard expansion for source files |
| `--age` int | Exclude files older than the specified age (in minutes) |

### Decryption Options
| Flag | Description |
|------|-------------|
| `--decrypt` | Enable file decryption (default: true) |
| `--decrypt_key` string | Specify the decryption key for 'winscp' or 'pgp' modes |
| `--decrypt_priv_key` string | Provide the decryption private key for 'ssh' mode (default: `/home/iychoi/.ssh/id_rsa`) |
| `--decrypt_temp` string | Set a temporary directory for file decryption (default: `/tmp`) |

### Low-Level Transfer Options
| Flag | Description |
|------|-------------|
| `--tcp_buffer_size` string | Specify the TCP socket buffer size (default: `1MB`) |

