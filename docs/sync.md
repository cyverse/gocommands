# Sync your data using Gocommands

You can sync your local data with iRODS (or reverse) using `Gocommands`.

You can use `get`, `put`, `bput`, `cp`, and `sync` subcommands for moving data.

## Get (Download) data from iRODS to local

`get` subcommand allows you to download data stored in iRODS.

To download a file or an entire directory:

```bash
gocmd get [irods_source] [local_destination]
```

For example, to download a whole directory `/iplant/home/iychoi/test_data` in iRODS to the current directory at local:

```bash
gocmd get /iplant/home/iychoi/test_data .
```

In order to use wildcards, add the `-w` option. For example, in order to download all CSV files from the `test_data` collection:

```bash
gocmd get -w '/iplant/home/iychoi/test_data/*.csv' .
```

Of course, you can use relative path from your iRODS current working directory to locate input. Use `pwd` and `cd` subcommand to display and change iRODS current working directory.

```bash
gocmd pwd
> /iplant/home/iychoi

gocmd get test_data .
```

### Useful flags

- `--progress`: Displays progress bars.
- `--diff`: Does not download a file if the file exists at local. Overwrites if the local file has different `size` or file `hash`.
- `--no_hash`: Works with `--diff`. Does not use file `hash` in file comparisons. This is a lot faster than using `hash` and useful if you don't change file content (like image files).
- `-f`: Downloads data in iRODS to local forcefully. Existing files at local will be overwritten.
- `--retry <num_retry>`: Retries the same command with given retry number if something goes wrong, like network failure. 
- `--retry_interval <seconds>`: Sets interval between each retry.


## Put (Upload) data from local to iRODS

`put` subcommand allows you to upload data to iRODS.

To upload a file or an entire directory:

```bash
gocmd put [local_source] [irods_destination]
```

For example, to upload a whole directory `test_data` at local to iRODS path `/iplant/home/iychoi/test_data`:

```bash
gocmd put test_data /iplant/home/iychoi/test_data
```

Of course, you can use relative path from your iRODS current working directory to locate output. Use `pwd` and `cd` subcommand to display and change iRODS current working directory.

```bash
gocmd pwd
> /iplant/home/iychoi

gocmd put test_data .
```

### Useful flags

- `--progress`: Displays progress bars.
- `--diff`: Does not upload a file if the file exists in iRODS. Overwrites if the iRODS file has different `size` or file `hash`.
- `--no_hash`: Works with `--diff`. Does not use file `hash` in file comparisons. This is a lot faster than using `hash` and useful if you don't change file content (like image files).
- `-f`: Uploads data at local to iRODS forcefully. Existing files in iRODS will be overwritten.
- `--no_replication`: Does not trigger iRODS data replication. Use this only if you know what this is.
- `--retry <num_retry>`: Retries the same command with given retry number if something goes wrong, like network failure. 
- `--retry_interval <seconds>`: Sets interval between each retry.

### Note

Parallel data upload is only available in iRODS 4.2.11+. So you will see that `Gocommands` does not use bandwidth efficiently for uploading files when the server runs lower versions of iRODS (like CyVerse Data Store). If you want to upload many small data, try `bput` subcommand to be explained below.



## Bulk put (Upload) data from local to iRODS

`bput` subcommand allows you to upload large datasets to iRODS.

The key ideas behind `bput` are:

- Creating `tar` bundles to combine small many files into large bundle files to make transfer more efficient.
- Transferring large bundles in parallel.
- Unbundling in iRODS server-side.


To upload a file or an entire directory:

```bash
gocmd bput [local_source] [irods_destination]
```

For example, to upload a whole directory `test_data` at local to iRODS path `/iplant/home/iychoi/test_data`:

```bash
gocmd bput test_data /iplant/home/iychoi/test_data
```

Of course, you can use relative path from your iRODS current working directory to locate output. Use `pwd` and `cd` subcommand to display and change iRODS current working directory.

```bash
gocmd pwd
> /iplant/home/iychoi

gocmd bput test_data .
```

### Useful flags

- `--progress`: Displays progress bars.
- `--diff`: Does not upload a file if the file exists in iRODS. Overwrites if the iRODS file has different `size` or file `hash`.
- `--no_hash`: Works with `--diff`. Does not use file `hash` in file comparisons. This is a lot faster than using `hash` and useful if you don't change file content (like image files).
- `-f`: Uploads data at local to iRODS forcefully. Existing files in iRODS will be overwritten.
- `--max_file_num`: Specifies the maximum number of files in a bundle. Default is 50.
- `--max_file_size`: Specifies the size threshold of a bundle. Default is 1GB.
- `--local_temp`: Specifies the local temporary directory to be used in creating bundle files. Default is `/tmp`.
- `--retry <num_retry>`: Retries the same command with given retry number if something goes wrong, like network failure. 
- `--retry_interval <seconds>`: Sets interval between each retry.


## Sync data between local and iRODS

`sync` subcommand allows you to sync datasets between local and iRODS. `sync` will transfers files only when they are not present or differet.

Input and output path arguments of `sync` can be either for local or iRODS. Paths with `i:` prefix are considered as iRODS paths. Paths with no prefix are considered as local paths.

To sync (upload) a file or an entire directory to iRODS:

```bash
gocmd sync [local_source] i:[irods_destination]
```

To sync (download) a file or an entire directory in iRODS to local:

```bash
gocmd sync i:[irods_source] [local_destination]
```


### Useful flags

- `--progress`: Displays progress bars.
- `--no_hash`: Does not use file `hash` in file comparisons. This is a lot faster than using `hash` and useful if you don't change file content (like image files).
- `-f`: Uploads data at local to iRODS forcefully. Existing files in iRODS will be overwritten.
- `--max_file_num`: Specifies the maximum number of files in a bundle. Default is 50.
- `--max_file_size`: Specifies the size threshold of a bundle. Default is 1GB.
- `--local_temp`: Specifies the local temporary directory to be used in creating bundle files. Default is `/tmp`.
- `--retry <num_retry>`: Retries the same command with given retry number if something goes wrong, like network failure. 
- `--retry_interval <seconds>`: Sets interval between each retry.

### Note

`sync` works exactly same as `get`, `bput`, and `copy`.

- `gocmd sync [local_source] i:[irods_destination]` works exactly same as `gocmd bput --diff [local_source] [irods_destination]`
- `gocmd sync i:[irods_source] [local_destination]` works exactly same as `gocmd get --diff [irods_source] [local_destination]`
- `gocmd sync i:[irods_source] i:[irods_destination]` works exactly same as `gocmd sync --diff [irods_source] [irods_destination]`
