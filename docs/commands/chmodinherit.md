# Change Access Permission Inheritance for Collections in iRODS

GoCommands allows you to modify access permission inheritance for collections (directories) in iRODS using the `chmodinherit` command. This command enables or disables access inheritance for a collection.

When inheritance is enabled for a collection, any new data objects or subcollections created within it will automatically inherit the same access permissions as the parent collection.

## Syntax
```sh
gocmd chmodinherit [flags]  
```

## Inheritance Options
- `inherit`: Enable access inheritance. Data objects and sub-collections inherit permissions from the parent collection.
- `noinherit`: Disable access inheritance. Data objects and sub-collections do not inherit permissions from the parent collection.

## Example Usage

1. **Enable inheritance for a collection:**
    ```sh
    gocmd chmodinherit inherit /iplant/home/myUser/dir
    ```

2. **Disable inheritance for a collection:**
    ```sh
    gocmd chmodinherit noinherit /iplant/home/myUser/dir
    ```

3. **Enable inheritance recursively for a collection and its subcollections:**
    ```sh
    gocmd chmodinherit -r inherit /iplant/home/myUser/dir
    ```

4. **Disable inheritance recursively for a collection and its subcollections:**
    ```sh
    gocmd chmodinherit -r noinherit /iplant/home/myUser/dir
    ```

## All Available Flags

| Flag                   | Description                                                                 |
|------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`  | Specify custom iRODS configuration file or directory path (default "/home/iychoi/.irods"). |
| `-d, --debug`          | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`           | Display help information about available commands and options.             |
| `--log_level string`   | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`          | Suppress all non-error output messages.                                    |
| `-r, --recursive`      | Recursively process operations for collections and their subcollections.   |
| `-s, --session int`    | Specify session identifier for tracking operations (default 834334).       |
| `-v, --version`        | Display version information.                                               |
