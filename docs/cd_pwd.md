# Current Working Collection (Directory) in iRODS

In iRODS, the **current working collection** is equivalent to the concept of a current working directory in traditional file systems. You can display or change your current working collection using GoCommands.

---

## Display the Current Working Collection

To display the current working collection in iRODS, use the `pwd` command. This command is similar to the Unix `pwd` command but tailored for iRODS.

### Syntax
```sh
gocmd pwd
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

1. To change to a specific collection:
    ```sh
    gocmd cd /myZone/home/myUser/mydata
    ```

    This changes your current working collection to `/myZone/home/myUser/mydata`.

2. To use a relative path from your current location:
    ```sh
    gocmd cd mydata
    ```

3. After changing the collection, you can confirm it with the `pwd` command:
    ```sh
    $ gocmd pwd
    /myZone/home/myUser/mydata
    ```

### Tips for Navigating Collections

1. **Return to Your Home Directory**

   - Using the full path:
     ```sh
     gocmd cd /myZone/home/myUser
     ```

   - Using no argument (defaults to home directory):
     ```sh
     gocmd cd
     ```

   - Using home path expansion with `~`:
     ```sh
     gocmd cd "~"
     ```
     > **Note:** The `~` must be quoted to prevent shell expansion by your local shell. Without quotes, it will expand to your local machine's home directory instead of your iRODS home directory.

2. **Move Up One Level**

   To move up one level in the directory tree:
   ```sh
   gocmd cd ..
   ```

3. **Navigate Using Absolute Paths**

   Always use absolute paths (e.g., `/myZone/home/myUser/target`) when you are unsure of your current location, as this ensures you navigate correctly.

4. **Check Your Current Location**

   Use `pwd` frequently to verify your current working collection before performing operations like uploading, downloading, or deleting files.
