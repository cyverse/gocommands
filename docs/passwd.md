# Change User Password in iRODS

The `passwd` command allows users to change their iRODS password. This command provides a secure way to update authentication credentials.

## Syntax
```sh
gocmd passwd [flags]
```

## Example Usage

To change your password:
```sh
gocmd passwd
```
This command will prompt you to enter your current password and then your new password twice for confirmation. The process is interactive to ensure security.

## Password Requirements

- The new password must be different from the current password.
- Password complexity requirements may vary depending on your iRODS server configuration.
- Typically, a strong password is recommended (e.g., combining uppercase and lowercase letters, numbers, and special characters).

## Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |

## Important Notes

- Ensure you remember your new password, as it will be required for future iRODS operations.
- If you encounter issues changing your password, contact your iRODS administrator.
- For security reasons, avoid using the same password across multiple systems.
- After changing your password, you may need to update it in any scripts or applications that use your iRODS credentials.
