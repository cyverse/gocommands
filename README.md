# gocommands
iRODS Command-line Tools written in Go


## Download pre-built binary
Please download binary file (bundled with `tar` or `zip`) at [https://github.com/cyverse/gocommands/releases](https://github.com/cyverse/gocommands/releases).
Be sure to download a binary for your target system architecture.

For Darwin-amd64 (Mac OS):
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.4.0/gocmd-v0.4.0-darwin-amd64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```

For Linux-amd64:
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.4.0/gocmd-v0.4.0-linux-amd64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```

For Linux-arm64:
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.4.0/gocmd-v0.4.0-linux-arm64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```


## Build from source (optional)
Use `make` to build `gocommands`. Binaries will be created on `./bin` directory.

```bash
make
```

## How to use

### Using the iCommands configuration
`gocommands` understands the iCommands' configuration files, `~/.irods/irods_environment.json`.
To create iCommands' configuration file, run `gocmd init` to create the configuration file under `~/.irods`.

```
gocmd init
```

If you already have iCommands' configuration files, you don't need any steps to do.

To check what configuration files you are loading
```
gocmd env
```

Run `ls`.
```
gocmd ls
```


### Using an external configuration file 
`gocommands` can read configuration from an `YAML` file.

Create `config.yaml` file using an editor and type in followings.
```yaml
irods_host: "data.cyverse.org"
irods_port: 1247
irods_user_name: "your username"
irods_zone_name: "iplant"
irods_user_password: "your password"
```

When you run `gocommands`, provide the configuration file's path with `-c` flag.
```bash
gocmd -c config.yaml ls
```

Some of field values, such as `irods_user_password` can be omitted if you don't want to put it in clear text. `gocommands` will ask you to type the missing field values in runtime.

### Using environmental variables 
`gocommands` can read configuration from environmental variables.

Set environmental variables
```bash
export IRODS_HOST="data.cyverse.org"
export IRODS_PORT=1247
export IRODS_USER_NAME="your username"
export IRODS_ZONE_NAME="iplant"
export IRODS_USER_PASSWORD="your password"
```

Then run `gocommands` with `-e` flag.
```bash
gocmd -e ls
```

Some of field values, such as `IRODS_USER_PASSWORD` can be omitted if you don't want to put it in clear text. `gocommands` will ask you to type the missing field values in runtime.
