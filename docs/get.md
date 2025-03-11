# Transferring Data Between Local Machine and iRODS

When working with iRODS, you can transfer data between your local machine and iRODS using three primary commands: `get`, `put`, and `bput`.

## Downloading Data from iRODS to Local Machine (`get`)

The `get` command is used to retrieve data stored in iRODS and download it to a local directory.

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

You can also omit the local destination path if you want to store the file in the current working directory:
```sh
gocmd get /myZone/home/myUser/myData1.obj
```

## Available Flags for `get`

### Transfer Mode
| Flag | Description |
|------|-------------|
| `--icat` | Transfer data via iCAT |
| `--redirect` | Redirect transfer to resource server |

### Parallel Transfer
| Flag | Description |
|------|-------------|
| `--single_threaded` | Transfer a file using a single thread |
| `--thread_num` int | Specify the number of transfer threads (default 5) |

### Differential Transfer
| Flag | Description |
|------|-------------|
| `--diff` | Transfer files with different content |
| `--no_hash` | Compare files without using hash, works with `--diff` |

### Data Verification
| Flag | Description |
|------|-------------|
| `-K, --verify_checksum` | Calculate and verify the checksum after transfer |

### File Management
| Flag | Description |
|------|-------------|
| `--delete` | Delete extra files in destination directory |
| `--delete_on_success` | Delete source file on success |
| `--report` string | Create a transfer report, specify path for file output; empty string or `-` outputs to stdout |

### User Interface and Display
| Flag | Description |
|------|-------------|
| `--progress` | Display progress bars |
| `--show_path` | Display full path for progress bars, works with `--progress` |
| `-f, --force` | Run forcefully (do not ask any prompt) |
| `--no_root` | Do not create root target directory |

### Source File Filters
| Flag | Description |
|------|-------------|
| `--exclude_hidden_files` | Exclude hidden files (starting with `.`) |
| `-w, --wildcard` | Enable wildcard expansion to search source files |
| `--age` int | Set the maximum age of the source in minutes |

### Decryption Options
| Flag | Description |
|------|-------------|
| `--decrypt` | Decrypt files (default true) |
| `--decrypt_key` string | Decryption key for 'winscp' and 'pgp' mode |
| `--decrypt_priv_key` string | Decryption private key for 'ssh' mode (default `/home/iychoi/.ssh/id_rsa`) |
| `--decrypt_temp` string | Specify temp directory path for decrypting files (default `/tmp`) |

### Low-level Transfer Options
| Flag | Description |
|------|-------------|
| `--tcp_buffer_size` string | Specify TCP socket buffer size (default `1MB`) |

