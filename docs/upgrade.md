# Upgrading GoCommands

Upgrading GoCommands to the latest release is straightforward:

1. Run the upgrade command:
   ```bash
   gocmd upgrade
   ```

2. You will need appropriate permissions to overwrite the existing GoCommands binary. On Unix-like systems, you might need to use `sudo` if GoCommands is installed in a system directory:
   ```bash
   sudo gocmd upgrade
   ```

This command will automatically fetch the latest release and update your installation.