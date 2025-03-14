# Display the Content of a Data Object in iRODS

To display the content of a data object in iRODS using GoCommands, you can use the `cat` command. This is similar to how you would use the `cat` command in a Unix-like environment to view the contents of a file.

### Syntax
```sh
gocmd cat <data-object>
```

### Example Usage
```sh
gocmd cat /myZone/home/myUser/hello.txt
```

This command will display the content of the specified data object. For instance, if the data object `hello.txt` contains the text "HELLO WORLD!", the output will be:
```sh
HELLO WORLD!
```
