# Managing Access Permissions

The `chmod` and `chmodinherit` commands allow users to manage access permissions for files and directories. The `ls -A` command displays the current access levels assigned to users for a given file or directory.

## Syntax  

### List access levels of users for a file or directory:  
```sh
gocmd ls -A <path>
```

### Change a user's or group's access level for a file or directory:  
```sh
gocmd chmod <access_level> <user_or_group> <path>
```

### Enable or disable access inheritance for a directory:  
```sh
gocmd chmodinherit <inherit_flag> <directory_path>
```

## Example Usage  

1. **List current access levels for a file or directory**:  
   ```sh
   gocmd ls -A /myZone/home/myUser/dir
   ```

2. **Grant a user write permission to a file**:  
   ```sh
   gocmd chmod write myUser /myZone/home/myUser/dir/file1
   ```

3. **Grant a user write permission to a directory and its contents**:  
   ```sh
   gocmd chmod -r write myUser /myZone/home/myUser/dir
   ```

4. **Grant a user read permission to a directory and its contents**:  
   ```sh
   gocmd chmod -r read myUser /myZone/home/myUser/dir
   ```

5. **Grant a user owner permission to a directory and its contents**:  
   ```sh
   gocmd chmod -r owner myUser /myZone/home/myUser/dir
   ```

6. **Enable access inheritance for all files and subdirectories**:  
   ```sh
   gocmd chmodinherit inherit /myZone/home/myUser/dir
   ```

7. **Disable access inheritance for all files and subdirectories**:  
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

## Available Inheritance Flags for `chmodinherit`  

| Flag | Description |
|------|-------------|
| `inherit` | Enable access inheritance—files and subdirectories inherit permissions from the parent directory |
| `noinherit` | Disable access inheritance—files and subdirectories do not inherit permissions from the parent directory |
