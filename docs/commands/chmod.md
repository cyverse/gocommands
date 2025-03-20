# Change Access Permissions for Data Objects and Collections in iRODS

GoCommands allows you to modify access permissions for data objects (files) and collections (directories) in iRODS using the `chmod` command. This is similar to how you would use the `chmod` command in a Unix-like environment to change the permissions of a file.


## Syntax
```sh
gocmd chmod [flags] <access-level> <user-or-group(#zone)> <data-object-or-collection>
```

### Access Levels
- `read`: Allows reading the object or collection
- `write`: Allows reading and modifying the object or collection
- `own`: Grants full control, including the ability to change permissions
- `null`: Removes all permissions

## Example Usage

1. **Grant a user read permission to a data object:**
    ```sh
    gocmd chmod read anotherUser /iplant/home/myUser/file.txt
    ```

2. **Grant a user from a different zone read permission to a data object:**
    ```sh
    gocmd chmod read anotherUser#anotherZone /iplant/home/myUser/file.txt
    ```

3. **Grant a user read permission to a collection and its contents:**
    ```sh
    gocmd chmod -r read anotherUser /iplant/home/myUser/dir
    ```

4. **Grant a user write permission to a collection and all its contents:**
    ```sh
    gocmd chmod -r write anotherUser /iplant/home/myUser/dir
    ```

5. **Grant a user owner permission to a collection and its contents:**
   ```sh
   gocmd chmod -r owner anotherUser /iplant/home/myUser/dir
   ```

6. **Remove access permission from a user to a collection and its contents:**
   ```sh
   gocmd chmod -r none anotherUser /iplant/home/myUser/dir
   ```

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/iychoi/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`                         | Display help information about available commands and options.             |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-r, --recursive`                    | Recursively process operations for collections and their contents.         |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 834334).       |
| `-v, --version`                      | Display version information.                                               |
