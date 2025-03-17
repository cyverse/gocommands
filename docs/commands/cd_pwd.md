# Current Working Collection (Directory) in iRODS

In iRODS, the **current working collection** is equivalent to the concept of a current working directory in traditional file systems. You can display or change your current working collection using GoCommands.

---

## Display the Current Working Collection

To display the current working collection in iRODS, use the `pwd` command. This command is similar to the Unix `pwd` command but tailored for iRODS.

### Syntax
```sh
gocmd pwd [flags]
```

### Example Usage
```sh
gocmd pwd
```

This command will output your current working collection, such as:
```sh
/myZone/home/myUser
```

By default, after configuring GoCommands, your current working collection is set to your **home directory**, which is typically located at:
```sh
//home/
```

---

## Change the Current Working Collection

To navigate to a different collection (directory) in iRODS, use the `cd` command. This allows you to move to a specific collection by specifying its path.

### Syntax
```sh
gocmd cd 
```

### Example Usage

1. **Change to a specific collection using an absolute path:**
    ```sh
    gocmd cd /myZone/home/myUser/mydata
    ```

    This changes your current working collection to `/myZone/home/myUser/mydata`.

2. **Use a relative path from your current location:**
   Assuming your current working collection is `/myZone/home/myUser`:
    ```sh
    gocmd cd mydata
    ```

3. **Confirm your current collection with the `pwd` command:**
    ```sh
    $ gocmd pwd
    /myZone/home/myUser/mydata
    ```

4. **Return to your home collection:**
   - Using the full path:
     ```sh
     gocmd cd /myZone/home/myUser
     ```

   - Using no argument (defaults to home collection):
     ```sh
     gocmd cd
     ```

   - Using home path expansion with `~`:
     ```sh
     gocmd cd "~"
     ```
     > **Note:** The `~` must be quoted to prevent shell expansion by your local shell. Without quotes, it will expand to your local machine's home directory instead of your iRODS home directory.

5. **Move up one level:**
   ```sh
   gocmd cd ..
   ```

## All Available Flags

| Flag                                | Description                                                                 |
|-------------------------------------|-----------------------------------------------------------------------------|
| `-c, --config string`               | Specify custom iRODS configuration file or directory path (default "/home/myUser/.irods"). |
| `-d, --debug`                        | Enable verbose debug output for troubleshooting.                           |
| `-h, --help`                         | Display help information about available commands and options.             |
| `--log_level string`                 | Set logging verbosity level (e.g., INFO, WARN, ERROR, DEBUG).              |
| `-q, --quiet`                        | Suppress all non-error output messages.                                    |
| `-s, --session int`                  | Specify session identifier for tracking operations (default 42938).        |
| `-v, --version`                      | Display version information.                                                |