# Installing GoCommands from Pre-built Binaries

GoCommands provides pre-built binaries for various operating systems and architectures. Choose the appropriate command for your system to install the latest version.

## macOS (Intel/AMD_64)

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-darwin-amd64.tar.gz | tar zxvf -
```

## macOS (Apple Silicon M1/M2)

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-darwin-arm64.tar.gz | tar zxvf -
```

## Linux (X86_64/AMD_64)

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-amd64.tar.gz | tar zxvf -
```

## Linux (ARM_64)

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-arm64.tar.gz | tar zxvf -
```

## Windows (Command Prompt/AMD_64)

```cmd
curl -L -s -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt && set /p GOCMD_VER=<gocmdv.txt
curl -L -s -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/%GOCMD_VER%/gocmd-%GOCMD_VER%-windows-amd64.zip && tar zxvf gocmd.zip && del gocmd.zip gocmdv.txt
```

## Windows (PowerShell/AMD_64)

```powershell
curl -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt ; $env:GOCMD_VER = (Get-Content gocmdv.txt)
curl -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/$env:GOCMD_VER/gocmd-$env:GOCMD_VER-windows-amd64.zip ; tar zxvf gocmd.zip ; del gocmd.zip ; del gocmdv.txt
```

## Manual Installation or Specific Versions

If you prefer to install a specific release version or want to download the binaries manually, please visit the GoCommands releases page:

[https://github.com/cyverse/gocommands/releases](https://github.com/cyverse/gocommands/releases)

From this page, you can select and download any available version of GoCommands for your operating system.