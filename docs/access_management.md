# Access Management

GoCommands provides features to manage access of users and groups to data in the Data Store. The `chmod` and `chmodinherit` commands allow users to manage access permissions for data objects (files) and collections (directories). The `ls -A` command displays the current access levels assigned to users for a given file or directory.

## :material-cog-outline: List Access Permissions of Users for a Data Object or Collection

```sh
gocmd ls -A <data-object-or-collection>
```

The `-A` flag in `ls` command displays access permissions in the result.

This command will show the data objects and collections along with their access control lists (ACLs). For example:
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

### Example Usage

1. **List current access levels for a data object or collection:**
   ```sh
   gocmd ls -A /myZone/home/myUser/mydata
   ```

## :material-cog-outline: Change a User's or Group's Access Permission for a Data Object or Collection

```sh
gocmd chmod <access-level> <user-or-group(#zone)> <data-object-or-collection>
```

### Access Levels

| Access Level | Description |
|-------------|-------------|
| `null` | Removes all permissions |
| `read` | Allows reading the object or collection |
| `write` | Allows reading and modifying the object or collection |
| `own` | Grants full control, including the ability to change permissions |

### Example Usage

1. **Grant a user read permission to a data object:**
    ```sh
    gocmd chmod read anotherUser /myZone/home/myUser/file.txt
    ```

2. **Grant a user from a different zone read permission to a data object:**
    ```sh
    gocmd chmod read anotherUser#anotherZone /myZone/home/myUser/file.txt
    ```

3. **Grant a user read permission to a collection and its contents:**
    ```sh
    gocmd chmod -r read anotherUser /myZone/home/myUser/dir
    ```

4. **Grant a user write permission to a collection and its contents:**
    ```sh
    gocmd chmod -r write anotherUser /myZone/home/myUser/dir
    ```

5. **Grant a user owner permission to a collection and its contents:**
   ```sh
   gocmd chmod -r owner anotherUser /myZone/home/myUser/dir
   ```

6. **Remove access permission from a user to a collection and its contents:**
   ```sh
   gocmd chmod -r none anotherUser /myZone/home/myUser/dir
   ```

## :material-cog-outline: Enable or Disable Access Permission Inheritance for a Collection

When inheritance is enabled for a collection, any new data objects or subcollections created within it will automatically inherit the same access permissions as the parent collection.

```sh
gocmd chmodinherit <inheritance_option> <collection_path>
```

### Inheritance Options

| Flag | Description |
|------|-------------|
| `inherit` | Enable access inheritance. Data objects and sub-collections inherit permissions from the parent collection |
| `noinherit` | Disable access inheritance. Data objects and sub-collections do not inherit permissions from the parent collection |

### Example Usage

1. **Enable inheritance for a collection:**
    ```sh
    gocmd chmodinherit inherit /myZone/home/myUser/dir
    ```

2. **Disable inheritance for a collection:**
    ```sh
    gocmd chmodinherit noinherit /myZone/home/myUser/dir
    ```

3. **Enable inheritance recursively for a collection and its subcollections:**
    ```sh
    gocmd chmodinherit -r inherit /myZone/home/myUser/dir
    ```

4. **Disable inheritance recursively for a collection and its subcollections:**
    ```sh
    gocmd chmodinherit -r noinherit /myZone/home/myUser/dir
    ```
