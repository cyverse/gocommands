# Transferring Data Between Local Machine and iRODS

When working with iRODS, you can transfer data between your local machine and iRODS using three primary commands: `get`, `put`, and `bput`.

## Downloading Data from iRODS to a Local Machine (`get`)

The `get` command retrieves data stored in iRODS and downloads it to a local directory.

### Syntax:
```sh
gocmd get <iRODS_source_path> <local_destination>
```

### Example:
To download a single file from iRODS to the current local directory:
```sh
gocmd get /myZone/home/myUser/myData1.obj .
```

You can also provide multiple source iRODS paths to download multiple files at once:
```sh
gocmd get /myZone/home/myUser/myData1.obj /myZone/home/myUser/myData2.obj .
```

If you want to store the file in the current working directory, you can omit the local destination path:
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
| `--single_threaded` | Transfer a file using a single thread |
| `--thread_num` int | Specify the number of transfer threads (default: 5) |

### Differential Transfer
| Flag | Description |
|------|-------------|
| `--diff` | Transfer files with different content |
| `--no_hash` | Compare files without using a hash (works with `--diff`) |

### Data Verification
| Flag | Description |
|------|-------------|
| `-K, --verify_checksum` | Calculate and verify the checksum after transfer |

### File Management
| Flag | Description |
|------|-------------|
| `--delete` | Delete extra files in the destination directory |
| `--delete_on_success` | Delete the source file upon successful transfer |
| `--report` string | Create a transfer report; specify the output file path. An empty string or `-` outputs to stdout |

### User Interface and Display
| Flag | Description |
|------|-------------|
| `--progress` | Display progress bars |
| `--show_path` | Display the full path for progress bars (works with `--progress`) |
| `-f, --force` | Run forcefully without prompting for confirmation |
| `--no_root` | Do not create a root target directory |

### Source File Filters
| Flag | Description |
|------|-------------|
| `--exclude_hidden_files` | Exclude hidden files (files starting with `.`) |
| `-w, --wildcard` | Enable wildcard expansion to search for source files |
| `--age` int | Exclude files older than the specified age in minutes |

### Decryption Options
| Flag | Description |
|------|-------------|
| `--decrypt` | Decrypt files (default: true) |
| `--decrypt_key` string | Decryption key for 'winscp' and 'pgp' modes |
| `--decrypt_priv_key` string | Decryption private key for 'ssh' mode (default: `/home/iychoi/.ssh/id_rsa`) |
| `--decrypt_temp` string | Specify a temporary directory for decrypting files (default: `/tmp`) |

### Low-level Transfer Options
| Flag | Description |
|------|-------------|
| `--tcp_buffer_size` string | Specify TCP socket buffer size (default: `1MB`) |

Stay tuned for additional sections on the `put` and `bput` commands!

