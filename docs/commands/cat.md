# Display the Content of a Data Object in iRODS

To display the content of a data object (file) in iRODS using GoCommands, you can use the `cat` command. This is similar to how you would use the `cat` command in a Unix-like environment to view the contents of a file.

## Syntax
```sh
gocmd cat [flags] <data-object>
```

## Example Usage
```sh
gocmd cat /myZone/home/myUser/hello.txt
```

This command will display the content of the specified data object. For instance, if the data object `hello.txt` contains the text "HELLO WORLD!", the output will be:
```sh
HELLO WORLD!
```

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`                         | Display help information about available commands and options.             |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-R, --resource string`              | Target specific iRODS resource server for operations.                     |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 42938).        |
| `-T, --ticket string`                | Specify the name of the ticket.                                            |
| `-v, --version`                      | Display version information.                                                |
