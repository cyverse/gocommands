# gocommands
iRODS Command-line Tools written in Go

## Build
Use `make` to build `gocommands`. Binaries will be created on `./bin` directory.

```bash
make
```

## How to use
`gocommands`'s configuration is compatible to `icommands`. 
Run `goinit` to configure iRODS account for access. 
Then, use other `gocommands`, such as `gols`, to access iRODS.

