# Upgrade GoCommands

The `upgrade` command allows you to upgrade GoCommands to the latest version, ensuring you have access to the newest features and bug fixes.

## Syntax
```sh
gocmd upgrade [flags]
```

## Example Usage

1. **Run the upgrade command:**
   ```bash
   gocmd upgrade
   ```  

   This command automatically downloads and installs the latest release.

   - If GoCommands is installed in a system directory, you may need administrative privileges. On Unix-like systems, use:
      ```bash
      sudo gocmd upgrade
      ```

2. **Check for the latest version available online without performing an update:**
   ```bash
   gocmd upgrade --check
   ```

## Useful Flags

1. **`--check`: Only check for the latest version without performing any updates.**
   ```sh
   gocmd upgrade --check
   ```

   This command checks for the latest available version of GoCommands without actually performing an upgrade. It provides information about the current installed version and the latest release version available.

   Example output:
   ```sh
   Current cilent version installed: v0.10.18
   Latest release version available for linux/amd64: v0.10.18
   Latest release URL: https://github.com/cyverse/gocommands/releases/tag/v0.10.18
   Current client version installed is up-to-date [v0.10.18]
   ```

## All Available Flags

| Flag                  | Description                                                                 |
|-----------------------|-----------------------------------------------------------------------------|
| `--check`             | Only check for the latest version without performing any updates.           |
| `-c, --config string` | Specify custom iRODS configuration file or directory path (default "/home/iychoi/.irods"). |
| `-d, --debug`         | Enable verbose debug output for troubleshooting.                            |
| `-h, --help`          | Display help information about available commands and options.              |
| `--log_level string`  | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).               |
| `-q, --quiet`         | Suppress all non-error output messages.                                     |
| `-s, --session int`   | Specify session identifier for tracking operations (default 42938).         |
| `-v, --version`       | Display version information.                                                |
