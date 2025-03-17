# Display the Current GoCommands Configuration

To display the current configuration of GoCommands, you can use the `env` command. This command provides a comprehensive overview of your GoCommands setup.

## Syntax
```sh
gocmd env [flags]
```

## Example Usage
```sh
gocmd env
```

This command will display the current configuration of GoCommands. The output will be similar to:

```sh
+------------------------------+--------------------------------------------------+
| Session Environment File     | /home/myUser/.irods/irods_environment.json.42938 |
| Environment File             | /home/myUser/.irods/irods_environment.json       |
| Authentication File          | /home/myUser/.irods/.irodsA                      |
| Host                         | my-irods.com                                     |
| Port                         | 1247                                             |
| Zone                         | myZone                                           |
| Username                     | myUser                                           |
| Client Zone                  | myZone                                           |
| Client Username              | myUser                                           |
| Default Resource             |                                                  |
| Current Working Dir          | /myZone/home/myUser                              |
| Home                         | /myZone/home/myUser                              |
| Default Hash Scheme          | SHA256                                           |
| Log Level                    | 0                                                |
| Authentication Scheme        | native                                           |
| Client Server Negotiation    | off                                              |
| Client Server Policy         | CS_NEG_REFUSE                                    |
| SSL CA Certification File    |                                                  |
| SSL CA Certification Path    |                                                  |
| SSL Verify Server            | hostname                                         |
| SSL Encryption Key Size      | 32                                               |
| SSL Encryption Key Algorithm | AES-256-CBC                                      |
| SSL Encryption Salt Size     | 8                                                |
| SSL Encryption Hash Rounds   | 16                                               |
+------------------------------+--------------------------------------------------+
```

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`          | Display help information about available commands and options.             |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`         | Suppress all non-error output messages.                                    |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).        |
| `-v, --version`       | Display version information.                                               |

