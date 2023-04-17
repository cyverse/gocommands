# Gocommands
iRODS Command-line Tools written in Go


## Download pre-built binary
Please download binary file (bundled with `tar` or `zip`) at [https://github.com/cyverse/gocommands/releases](https://github.com/cyverse/gocommands/releases).
Be sure to download a binary for your target system architecture.

For Darwin-amd64 (Mac OS):
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.6.5/gocmd-v0.6.5-darwin-amd64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```

For Linux-amd64:
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.6.5/gocmd-v0.6.5-linux-amd64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```

For Linux-arm64:
```bash
curl -L -o gocmd.tar.gz https://github.com/cyverse/gocommands/releases/download/v0.6.5/gocmd-v0.6.5-linux-arm64.tar.gz && \
tar zxvf gocmd.tar.gz && rm gocmd.tar.gz
```


## How to use

### Using the iCommands configuration
`Gocommands` understands the iCommands' configuration files, `~/.irods/irods_environment.json`.
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
`Gocommands` can read configuration from an `YAML` file.

Create `config.yaml` file using an editor and type in followings.
```yaml
irods_host: "data.cyverse.org"
irods_port: 1247
irods_user_name: "your username"
irods_zone_name: "iplant"
irods_user_password: "your password"
```

When you run `Gocommands`, provide the configuration file's path with `-c` flag.
```bash
gocmd -c config.yaml ls
```

Some of field values, such as `irods_user_password` can be omitted if you don't want to put it in clear text. `Gocommands` will ask you to type the missing field values in runtime.

### Using environmental variables 
`Gocommands` can read configuration from environmental variables.

Set environmental variables
```bash
export IRODS_HOST="data.cyverse.org"
export IRODS_PORT=1247
export IRODS_USER_NAME="your username"
export IRODS_ZONE_NAME="iplant"
export IRODS_USER_PASSWORD="your password"
```

Then run `Gocommands` with `-e` flag.
```bash
gocmd -e ls
```

Some of field values, such as `IRODS_USER_PASSWORD` can be omitted if you don't want to put it in clear text. `Gocommands` will ask you to type the missing field values in runtime.

## Troubleshooting

### Getting `SYS_NOT_ALLOWED` error

`put`, `bput`, or `sync` subcommands throw `SYS_NOT_ALLOWED` error if iRODS server does not support data replication. To disable data replication, use `--no_replication` flag.


## License

Copyright (c) 2010-2023, The Arizona Board of Regents on behalf of The University of Arizona

All rights reserved.

Developed by: CyVerse as a collaboration between participants at BIO5 at The University of Arizona (the primary hosting institution), Cold Spring Harbor Laboratory, The University of Texas at Austin, and individual contributors. Find out more at http://www.cyverse.org/.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

 * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
 * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
 * Neither the name of CyVerse, BIO5, The University of Arizona, Cold Spring Harbor Laboratory, The University of Texas at Austin, nor the names of other contributors may be used to endorse or promote products derived from this software without specific prior written permission.


Please check [LICENSE](https://github.com/cyverse/gocommands/tree/master/LICENSE) file.