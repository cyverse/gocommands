# List Data Objects (Files) and Collections (directories) in iRODS

To list data objects (files) and collections (directories) in iRODS using GoCommands, you can use the `ls` command. This command is similar to the Unix `ls` command but is adapted for use with iRODS.

## Syntax
```sh
gocmd ls <data-object-or-collection>... [flags]
```

## Example Usage

1. List the contents of a collection, `/myZone/home/myUser/mydata`:
    ```sh
    gocmd ls /myZone/home/myUser/mydata
    ```

    This will display the data objects and collections in the `/myZone/home/myUser/mydata` collection:
    ```sh
    /myZone/home/myUser/mydata:
      file1.bin
      file2.bin
      C- /myZone/home/myUser/mydata/subdir1
    ```

    The `C-` prefix indicates that the item is a collection (directory).
    
2. Show a specific data-object, `/myZone/home/myUser/mydata/file1.bin`:
    ```sh
    gocmd ls /myZone/home/myUser/mydata/file1.bin
    ```

    This will display the data object `/myZone/home/myUser/mydata/file1.bin`:
    ```sh
      /myZone/home/myUser/mydata/file1.bin
    ```

    This is useful for checking if a specific data object exists.

3. List the contents of multiple collections, `/myZone/home/myUser/mydata1` and `/myZone/home/myUser/mydata2`:
    ```sh
    gocmd ls /myZone/home/myUser/mydata1 /myZone/home/myUser/mydata2
    ```

    This will display data objects and collections in both `/myZone/home/myUser/mydata1` and `/myZone/home/myUser/mydata2`:
    ```sh
    /myZone/home/myUser/mydata1:
      file1.bin
      file2.bin
      C- /myZone/home/myUser/mydata/subdir1

    /myZone/home/myUser/mydata2:
      file3.bin
    ```

4. List the contents of the current working collection:
    ```sh
    gocmd ls
    ```

    This command will display the data objects and collections within your current working collection. By default, the current working collection is your home collection, `/myZone/home/myUser`.
    ```sh
    /myZone/home/myUser:
      C- /myZone/home/myUser/mydata
      C- /myZone/home/myUser/mydata1
      C- /myZone/home/myUser/mydata2
      C- /myZone/home/myUser/mydata_large
      C- /myZone/home/myUser/mydata_sort
    ```

## Useful Flags

1. `-A`, `--access`: Display access control lists for data-objects and collections
    ```sh
    gocmd ls -A /myZone/home/myUser/mydata
    ```

    This command will show the data objects and collections within `/myZone/home/myUser/mydata`, along with their access control lists (ACLs):
    ```sh
    /myZone/home/myUser/mydata:
            ACL - g:rodsadmin#myZone:own	myUser#myZone:own
            Inheritance - Disabled
      file1.bin
            ACL - g:rodsadmin#myZone:own	myUser#myZone:own
      file2.bin
            ACL - g:rodsadmin#myZone:own	myUser#myZone:own
      C- /myZone/home/myUser/mydata/subdir1
    ```

    - The `g:` prefix in the ACL username indicates that the user is a group.
    - The ACL is displayed in the `username#zone:access_level` format.
    - Most common access levels are:
        - `read_object`: Allows read access to the data object or collection.
        - `modify_object`: Allows modification (write) of the data object or collection.
        - `own`: Grants ownership of the data object or collection.

2. `-l`, `--long`: Display results in long format with additional details
    ```sh
    gocmd ls -l /myZone/home/myUser/mydata
    ```

    This command will show the data objects and collections within `/myZone/home/myUser/mydata`, along with their additional details:
    ```sh
    /myZone/home/myUser/mydata:
      myUser	0	demoRes1;rs1	436	2024-04-02.13:36	&	file1.bin
      myUser	1	demoRes2;rs2	436	2024-04-02.13:36	&	file1.bin
      myUser	0	demoRes1;rs1	700	2024-04-02.17:15	&	file2.bin
      myUser	1	demoRes2;rs2	700	2024-04-02.17:15	&	file2.bin
      C- /myZone/home/myUser/mydata/subdir1
    ```

    - Each output line for a data object represents a replica. If iRODS is configured to create multiple replicas, you will see one line for each replica of the data object. For example, if two replicas are created, two lines will be displayed for each file.
    - Each line is shown in the `owner replica_id resource_server size creation_time replica_state name` format.
    - Possible replica states are:
        - `&`: Good
        - `X`: Stale
        - `?`: Unknown

3. `-L`, `--verylong`: Display results in very long format with comprehensive information
    ```sh
    gocmd ls -L /myZone/home/myUser/mydata
    ```

    This command will display the data objects and collections within `/myZone/home/myUser/mydata`, along with detailed information about each item:
    ```sh
    /myZone/home/myUser/mydata:
      myUser	0	demoRes1;rs1	436	2024-04-02.13:36	&	file1.bin
        9439fcb148c9a6709d9575440eedfd5d	/irods_vault/rs1/home/myUser/myData/file1.bin
      myUser	1	demoRes2;rs2	436	2024-04-02.13:36	&	file1.bin
        9439fcb148c9a6709d9575440eedfd5d	/irods_vault/rs2/home/myUser/myData/file1.bin
      myUser	0	demoRes1;rs1	700	2024-04-02.17:15	&	file2.bin
        400ac2b9d16ffa7041b0a494752ba826	/irods_vault/rs1/home/myUser/myData/file2.bin
      myUser	1	demoRes2;rs2	700	2024-04-02.17:15	&	file2.bin
        400ac2b9d16ffa7041b0a494752ba826	/irods_vault/rs2/home/myUser/myData/file2.bin
      C- /myZone/home/myUser/mydata/subdir1
    ```

    - The flag adds a new output line for each replica, displaying its checksum and physical location in the iRODS resource server.
    - The checksum can be generated using various hash algorithms, such as `MD5` or `SHA256`. If the algorithm used is not `MD5`, the checksum string will include the algorithm as a prefix. For example: `sha256:84bae6e26c2d40fc02a0dd266dccac0c34a2023d1a2ba8fea59dd11693445828`.

4. `-H`, `--human_readable`: Display data object sizes in human-readable units (KB, MB, GB).
    This flag must be used with the `-l` or `-L` flags, as they display data object sizes.
    ```sh
    gocmd ls -H -l /myZone/home/myUser/mydata_large
    ```

    The command above lists the data objects and collections within `/myZone/home/myUser/mydata_large`, displaying their sizes in human-readable units.
    ```sh
    /myZone/home/myUser/mydata_large:
      myUser	0	demoRes1;rs1	72 MB   2025-03-05.14:46	&	large_file1.bin
      myUser	1	demoRes2;rs2	72 MB   2025-03-05.14:46	&	large_file1.bin
      myUser	0	demoRes1;rs1	5.0 GB  2025-02-19.10:32	&	large_file2.bin
      myUser	1	demoRes2;rs2	5.0 GB  2025-02-19.10:32	&	large_file2.bin
    ```

5. `-S`, `--sort`: Sort data objects and collections in ascending order by `name`, `size`, `time`, or `ext`.
    ```sh
    gocmd -S name /myZone/home/myUser/mydata_sort
    ```

    The command above lists the data objects and collections within `/myZone/home/myUser/mydata_sort`, sorting them in ascending (alphabetical) order by name.
    ```sh
    /myZone/home/myUser/mydata_sort:
      Date.txt
      apple.txt
      banana.txt
      orange.txt
      watermelon.txt
      C- /iplant/home/iychoi/mydata_sort/Radish
      C- /iplant/home/iychoi/mydata_sort/cabbage
      C- /iplant/home/iychoi/mydata_sort/zucchini
    ```

    - Capital letters appear before lowercase letters in the ASCII table, so they are displayed first in the sorted list.
    - Collections are always displayed at the bottom of the list.
    - Available sort fields:
      - `name`: Sort by name.
      - `size`: Sort by size.
      - `time`: Sort by creation time first, with modification time as a secondary criterion.
      - `ext`: Sort by file extension.

6. `--reverse_sort`: Sort data objects and collections in reverse order.
    This flag is often used with the `-S` flag.
    ```sh
    gocmd ls -S name --reverse_sort /myZone/home/myUser/mydata_sort
    ```

    The command above lists the data objects and collections within `/myZone/home/myUser/mydata_sort`, sorting them in descending order by name.
    ```sh
    /myZone/home/myUser/mydata_sort:
      watermelon.txt
      orange.txt
      banana.txt
      apple.txt
      Date.txt
      C- /iplant/home/iychoi/mydata_sort/zucchini
      C- /iplant/home/iychoi/mydata_sort/cabbage
      C- /iplant/home/iychoi/mydata_sort/Radish
    ```

7. `-w`, `--wildcard`: Enable wildcard expansion to search for source files.
    ```sh
    gocmd ls -w /myZone/home/myUser/mydata_sort/*a*
    ```

    The command above lists data objects and collections within `/myZone/home/myUser/mydata_sort` that contain the letter `a` in their names.
    ```sh
      /iplant/home/iychoi/mydata_sort/Date.txt
      /iplant/home/iychoi/mydata_sort/apple.txt
      /iplant/home/iychoi/mydata_sort/banana.txt
      /iplant/home/iychoi/mydata_sort/orange.txt
      /iplant/home/iychoi/mydata_sort/watermelon.txt
    /iplant/home/iychoi/mydata_sort/Radish:

    /iplant/home/iychoi/mydata_sort/cabbage:

    ```

    - Data objects are always displayed with their full paths.
    - Collections in the path are visited recursively.
    - Available wildcards:
      - `?`: Matches a single character.
      - `*`: Matches any number of characters.
      - `[abc]`: Matches any one of the specified characters.


## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-A, --access`                      | Display access control lists for data-objects and collections               |
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default `"/home/iychoi/.irods"`) |
| `-d, --debug`                       | Enable verbose debug output for troubleshooting                             |
| `--decrypt`                         | Enable file decryption (default true)                                       |
| `--decrypt_key string`              | Specify the decryption key for 'winscp' or 'pgp' modes                      |
| `--decrypt_priv_key string`         | Provide the decryption private key for 'ssh' mode (default `"/home/iychoi/.ssh/id_rsa"`) |
| `--decrypt_temp string`             | Set a temporary directory for file decryption (default `"/tmp"`)            |
| `--exclude_hidden_files`            | Skip files and directories that start with '.'                              |
| `-h, --help`                        | Display help information about available commands and options               |
| `-H, --human_readable`              | Show file sizes in human-readable units (KB, MB, GB)                       |
| `--log_level string`                | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG)                |
| `-l, --long`                        | Display results in long format with additional details                      |
| `--no_decrypt`                      | Disable file decryption forcefully                                          |
| `-q, --quiet`                       | Suppress all non-error output messages                                      |
| `-R, --resource string`             | Target specific iRODS resource server for operations                        |
| `--reverse_sort`                    | Sort results in reverse order                                               |
| `-s, --session int`                 | Specify session identifier for tracking operations (default `313985`)       |
| `-S, --sort string`                 | Sort results by: name, size, time, or ext (default `name`)                 |
| `-T, --ticket string`               | Specify the name of the ticket                                              |
| `-v, --version`                     | Display version information                                                |
| `-L, --verylong`                    | Display results in very long format with comprehensive information           |
| `-w, --wildcard`                    | Enable wildcard expansion to search for source files                        |
