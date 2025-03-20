# Managing Access Permissions

The `chmod` and `chmodinherit` commands allow users to manage access permissions for data objects (files) and collections (directories). The `ls -A` command displays the current access levels assigned to users for a given data object or collection.

## Syntax  

### List access levels of users for a data object or collection:  
```sh
gocmd ls -A <path>
```

### Change a user's or group's access level for a data object or collection:  
```sh
gocmd chmod <access_level> <user_or_group> <path>
```

### Enable or disable access inheritance for a collection:  
```sh
gocmd chmodinherit <inherit_flag> <collection_path>
```

## Example Usage  

1. **List current access levels for a data object or collection:**
   ```sh
   gocmd ls -A /myZone/home/anotherUser/dir
   ```

2. **Grant a user write permission to a data object:**
   ```sh
   gocmd chmod write anotherUser /myZone/home/myUser/dir/file1
   ```

3. **Grant a user write permission to a collection and its contents:**
   ```sh
   gocmd chmod -r write anotherUser /myZone/home/myUser/dir
   ```

4. **Grant a user read permission to a collection and its contents:**
   ```sh
   gocmd chmod -r read anotherUser /myZone/home/myUser/dir
   ```

5. **Grant a user owner permission to a collection and its contents:**
   ```sh
   gocmd chmod -r owner anotherUser /myZone/home/myUser/dir
   ```

6. **Remove 

6. **Enable access inheritance for all data objects and subdirectories:**
   ```sh
   gocmd chmodinherit inherit /myZone/home/myUser/dir
   ```

7. **Disable access inheritance for all data objects and subdirectories:**
   ```sh
   gocmd chmodinherit noinherit /myZone/home/myUser/dir
   ```

## Available Access Levels for `chmod`  

| Access Level | Description |
|-------------|-------------|
| `null` | Remove all access permissions |
| `read` | Grant read access |
| `write` | Grant write access |
| `own` | Grant ownership (full control) |

## Available Inheritance Value for `chmodinherit`  

| Flag | Description |
|------|-------------|
| `inherit` | Enable access inheritance. Data objects and sub-collections inherit permissions from the parent collection |
| `noinherit` | Disable access inheritance. Data objects and sub-collections do not inherit permissions from the parent collection |
