# Current Working Collection (Directory) in iRODS

## Display the Current Working Collection

To display the current working collection (directory) in iRODS, you can use the `pwd` command. This command is similar to the Unix `pwd` command but is adapted for iRODS.

### Syntax
```sh
gocmd pwd
```

### Example Usage
```sh
gocmd pwd
```

This command displays your current working collection in iRODS, like this:
```sh
/myZone/home/myUser
```

After configuring GoCommands, your current working collection will be set to your home directory. Your home directory is `/<zone>/home/<username>`.

## Change the Current Working Collection

If you need to change the current working collection, you can use the `cd` command. This command allows you to navigate through iRODS collections by specifying the target collection path.

### Syntax
```sh
gocmd cd <collection>
```

### Example Usage
```sh
gocmd cd /myZone/home/myUser/mydata
```

This command changes your current working collection in iRODS to `/myZone/home/myUser/mydata`. You can also use a relative path to the destination from the current working collection:
```sh
gocmd cd mydata
```

After changing the current working collection to `/myZone/home/myUser/mydata`, the `pwd` command will display the updated path:
```sh
$ gocmd pwd
/myZone/home/myUser/mydata
```

### Tips
To change the current working collection back to your home directory, you have several options:
1. **Using the full path:**
```sh
gocmd cd /myZone/home/myUser
```

2. **Using the relative path:**
```sh
gocmd cd ..
```

3. **Using home path expansion with `~`:**
```sh
gocmd cd "~"
```
- **Note:** The `~` is quoted to prevent shell expansion. Without the quotes, shell expansion will replace `~` with your home path on your local machine.

4. **Passing no argument:**
```sh
gocmd cd
```
