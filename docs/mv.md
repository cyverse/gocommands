# Move (Rename) Data Objects and Collections in iRODS

The `mv` command allows you to move or rename data objects and collections in iRODS. This command is similar to the Unix `mv` command but is adapted for use with iRODS.

## Syntax
```sh
gocmd mv <data-object-or-collection>... <target-data-object-or-collection> [flags]
```

## Example Usage

1. To rename a data object:
    ```sh
    gocmd mv /myZone/home/myUser/oldfile.txt /myZone/home/myUser/newfile.txt
    ```

2. To move a data object to a different collection:
    ```sh
    gocmd mv /myZone/home/myUser/file.txt /myZone/home/myUser/subcollection/
    ```

3. To rename a collection:
    ```sh
    gocmd mv /myZone/home/myUser/oldcollection /myZone/home/myUser/newcollection
    ```

4. To move multiple data objects:
    ```sh
    gocmd mv /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt /myZone/home/myUser/targetcollection/
    ```

## Important Notes

1. When moving data objects or collections, ensure you have the necessary permissions in both the source and target locations.

2. Moving large collections or numerous data objects may take considerable time, depending on the size and number of files involved.

3. If you're moving data between different storage resources, the physical files will be transferred. This operation might be time-consuming for large data sets.

4. Always double-check your command before executing, especially when moving or renaming important data.
