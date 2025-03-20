# Data Management

GoCommands offers a variety of commands to help you manage your data in iRODS. In iRODS, `file` and `directory` are treated as `data objects` and `collections`, respectively. It's perfectly fine to consider these terms interchangeable.

## Display the Current Working Collection

In iRODS, the **current working collection** is equivalent to the concept of a current working directory in traditional file systems. You can display or change your current working collection using GoCommands.

```sh
gocmd pwd
```

By default, after configuring GoCommands, your current working collection is set to your **home directory**, which is typically located at:
```sh
/<Zone Name>/home/<Username>
```

> **Note:** Paths in iRODS always start with the zone name `/myZone`.

## Change the Current Working Collection

1. **Change to a specific collection using an absolute path:**
   ```sh
   gocmd cd /myZone/home/myUser/mydata
   ```

   This changes your current working collection to `/myZone/home/myUser/mydata`.

2. **Use a relative path from your current location:**
   Assuming your current working collection is `/myZone/home/myUser`:
   ```sh
   gocmd cd mydata
   ```

3. **Return to your home collection:**
   ```sh
   gocmd cd "~"
   ```

   > **Note:** The `~` must be quoted to prevent shell expansion by your local shell. Without quotes, it will expand to your local machine's home directory instead of your Data Store home directory.

4. **Move up one level:**
   ```sh
   gocmd cd ..
   ```

## List Data Objects (files) and Collections (directories) in iRODS

1. **List the content of a collection:**
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

2. **List the content of the current working collection:**
   ```sh
   gocmd ls
   ```

3. **List the contents of a collection in long format with additional details:**
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

4. **List the contents of a collection with their access control lists:**
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

## Make a Collections (directories) in iRODS

1. **Create a new collection:**
   ```sh
   gocmd mkdir /myZone/home/myUser/newCollection
   ```

2. **Create parent collections if they do not exist:**
   ```sh
   gocmd mkdir -p /myZone/home/myUser/parentCollection/newCollection
   ```
   This command creates the `newCollection` along with its parent collection `parentCollection` if it does not already exist.


## Upload Data Objects (files) and Collections (directories) to iRODS

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

3. **Upload with progress bars:**
   ```sh
   gocmd put --progress /local/path/largefile.dat /myZone/home/myUser/
   ```

4. **Force upload:**
   ```sh
   gocmd put -f /local/path/largefile.dat /myZone/home/myUser/
   ```

   This command overwrites the existing file in iRODS without prompting.

5. **Upload and verify checksum:**
   ```sh
   gocmd put -K /local/path/important_data.txt /myZone/home/myUser/
   ```

   This command uploads the file and verifies its integrity by calculating a checksum during transfer.

6. **Upload only different or new contents:**
   ```sh
   gocmd put --diff /local/dir /myZone/home/myUser/
   ```

   This command uploads only files that are different or don't exist in the destination. It compares file sizes and checksums to determine which files need updating.

7. **Upload via iCAT:**
   ```sh
   gocmd put --icat /local/dir /myZone/home/myUser/
   ```

   This command uses iCAT as a transfer broker, useful when direct access to the resource server is unstable.

8. **Upload via resource server:**
   ```sh
   gocmd put --redirect /local/dir /myZone/home/myUser/
   ```

   This command bypasses the iCAT server for data transfer, directly accessing the specified resource server for optimized performance.

## Download Data Objects (files) and Collections (directories) From iRODS

1. **Download a data object to a specific local path:**
   ```sh
   gocmd get /myZone/home/myUser/file.txt /local/path/file_new_name.txt
   ```

   This command downloads the data object `/myZone/home/myUser/file.txt` and saves it as `/local/path/file_new_name.txt`.

2. **Download a collection to a specific local path:**
   ```sh
   gocmd get /myZone/home/myUser/dir /local/path/
   ```

   This command downloads the collection `/myZone/home/myUser/dir` and its contents to `/local/path`. A new directory named `dir` will be created under `/local/path`, resulting in `/local/path/dir` containing all the downloaded files and subdirectories.

3. **Download with progress bars:**
   ```sh
   gocmd get --progress /myZone/home/myUser/largefile.dat /local/dir/
   ```

4. **Force download:**
   ```sh
   gocmd get -f /myZone/home/myUser/largefile.dat .
   ```

   This command overwrites the local file without prompting if it already exists.

5. **Download and verify checksum:**
   ```sh
   gocmd get -K /myZone/home/myUser/important_data.txt .
   ```

   This command downloads the file and verifies its integrity by calculating the checksum after download and comparing it with the original in iRODS. This ensures data consistency and detects any corruption during transfer.

6. **Download only different or new contents:**
   ```sh
   gocmd get --diff /myZone/home/myUser/dir /local/dir
   ```

   This command downloads the source collection to the local directory, transferring only files that are different or don't exist locally. It compares file sizes and checksums to determine which files need updating, making the transfer more efficient by skipping unchanged files.

7. **Download via iCAT:**
   ```sh
   gocmd get --icat /myZone/home/myUser/dir /local/dir
   ```

   This command uses iCAT as a transfer broker, which is useful when direct access to the resource server is unstable. It ensures reliable data transfer by routing through the iCAT server

8. **Download via resource server:**
   ```sh
   gocmd get --redirect /myZone/home/myUser/dir /local/dir
   ```

   This command bypasses the iCAT server for data transfer, directly accessing the specified resource server. It optimizes performance for large files by direct connection to the resource server.


9. **Download with wildcard:** 
   ```sh
   gocmd get -w /myZone/home/myUser/dir/file*.txt /local/dir
   ```

   This command downloads all data objects matching the pattern "file*.txt" from the specified collection to the local directory.


## Remove Data Objects (files) or Collections (directories) From iRODS

1. **Remove a single data object:**
   ```sh
   gocmd rm /myZone/home/myUser/file.txt
   ```

2. **Remove an empty collection:**
   ```sh
   gocmd rmdir /myZone/home/myUser/emptyCollection
   ```

3. **Remove a collection and its contents recursively:**
   ```sh
   gocmd rm -r /myZone/home/myUser/parentCollection
   ```

4. **Force remove a collection and its contents recursively:**
   ```sh
   gocmd rm -rf /myZone/home/myUser/parentCollection
   ```

## Move/Rename Data Objects (files) or Collections (directories) in iRODS

1. **Rename a data object:**
   ```sh
   gocmd mv /myZone/home/myUser/oldfile.txt /myZone/home/myUser/newfile.txt
   ```

2. **Move a data object to a different collection:**
   ```sh
   gocmd mv /myZone/home/myUser/file.txt /myZone/home/myUser/subcollection/
   ```

3. **Rename a collection:**
   ```sh
   gocmd mv /myZone/home/myUser/oldcollection /myZone/home/myUser/newcollection
   ```

4. **Move multiple data objects with wildcard:**
   ```sh
   gocmd mv -w /myZone/home/myUser/*.txt /myZone/home/myUser/targetcollection/
   ```

   This command will move all data objects with the `.txt` extension from the /myZone/home/myUser/ collection to the `targetcollection`. The asterisk (*) wildcard matches any number of characters in the filename.

   You can use more specific wildcard patterns for precise file selection, such as `file*.txt` to move all text files starting with "file".


## :material-list-box-outline: Additional Resources

For detailed information on GoCommands, refer to the command-specific documentation available for each command:

- [init](commands/init.md): Initialize GoCommands configuration
- [env](commands/env.md): Display or modify environment variables
- [passwd](commands/passwd.md): Change user password
- [cd and pwd](commands/cd_pwd.md): Change and print working directory
- [ls](commands/ls.md): List directory contents
- [touch](commands/touch.md): Create empty files or update timestamps
- [mkdir](commands/mkdir.md): Create directories
- [rm](commands/rm.md): Remove files or directories
- [rmdir](commands/rmdir.md): Remove directories
- [mv](commands/mv.md): Move or rename files and directories
- [cp](commands/cp.md): Copy files or directories
- [cat](commands/cat.md): Display contents of a file
- [get](commands/get.md): Download files from iRODS
- [put](commands/put.md): Upload files to iRODS
- [bput](commands/bput.md): Bulk upload files to iRODS
- [sync](commands/sync.md): Synchronize local and remote directories
- [chmod](commands/chmod.md): Change access permission of files or directories
- [chmodinherit](commands/chmodinherit.md): Change access permission inheritance of directories
- [lsmeta](commands/lsmeta.md): List metadata of data objects, collections, resources, or users in iRODS
- [addmeta](commands/addmeta.md): Add metadata to data objects, collections, resources, or users in iRODS
- [rmmeta](commands/rmmeta.md): Remove metadata from data objects, collections, resources, or users in iRODS
- [copy-sftp-id](commands/copy-sftp-id.md): Configure SFTP Public-key Authentication
- [svrinfo](commands/svrinfo.md): Display server information
- [ps](commands/ps.md): Display current iRODS sessions
- [upgrade](commands/upgrade.md): Upgrade GoCommands
