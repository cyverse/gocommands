# Installing GoCommands from Pre-built Binaries

<figure markdown>
  ![!ds](../assets/ds/datastore_plchldr.png){ width="200" }
</figure>

GoCommands provides pre-built binaries for various operating systems and architectures. Choose the appropriate command for your system to install the latest version.

## macOS  

macOS runs on a variety of Apple devices, including MacBook, MacBook Pro, MacBook Air, iMac, Mac mini, Mac Studio, and Mac Pro. Depending on the model, it may use either an Intel/AMD 64-bit CPU or Apple Silicon (M1/M2). Follow the appropriate installation instructions based on your processor.

If you're unsure which CPU architecture your Mac uses, run the following command in the terminal:

```bash
uname -p
```

This will return `aarch64` / `arm64` for Apple Silicon (M1/M2) or `x86_64` for Intel-based Macs.

### Intel 64-bit
Intel processors were used in Mac devices released before 2020. If you're using an older Mac, install GoCommands with the following command:  

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-darwin-amd64.tar.gz | tar zxvf -
```

### Apple Silicon (M1/M2)  
Apple introduced its custom Silicon chips (M1, M2) in 2020, replacing Intel processors in newer Mac models. If your device runs on Apple Silicon, use the following command to install GoCommands:  

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-darwin-arm64.tar.gz | tar zxvf -
```

## Linux

Linux supports a wide range of CPU architectures. Follow the appropriate installation instructions based on your processor type.  

If you're unsure which CPU architecture your system is using, run the following command in the terminal:  

```bash
uname -p
```  

This command will return the architecture type, such as `x86_64` for 64-bit Intel/AMD processors, `i386` / `i686` for 32-bit Intel/AMD processors, `aarch64` / `arm64` for 64-bit ARM processors, or `arm` for 32-bit ARM processors.

### Intel/AMD 64-bit

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-amd64.tar.gz | tar zxvf -
```

### Intel/AMD 32-bit

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-386.tar.gz | tar zxvf -
```

### ARM 64-bit

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-arm64.tar.gz | tar zxvf -
```

### ARM 32-bit

```bash
GOCMD_VER=$(curl -L -s https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt); \
curl -L -s https://github.com/cyverse/gocommands/releases/download/${GOCMD_VER}/gocmd-${GOCMD_VER}-linux-arm.tar.gz | tar zxvf -
```

## Windows  

Windows primarily runs on Intel/AMD CPU architectures. Most modern systems use 64-bit Intel/AMD processors, while very old systems may run on 32-bit processors.  

Windows includes two main terminal applications: `Command Prompt (CMD)` and `PowerShell`. Follow the appropriate installation instructions based on your processor type and preferred terminal.

### Intel/AMD 64-bit

#### Command Prompt

```cmd
curl -L -s -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt && set /p GOCMD_VER=<gocmdv.txt
curl -L -s -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/%GOCMD_VER%/gocmd-%GOCMD_VER%-windows-amd64.zip && tar zxvf gocmd.zip && del gocmd.zip gocmdv.txt
```

#### PowerShell

```powershell
curl -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt ; $env:GOCMD_VER = (Get-Content gocmdv.txt)
curl -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/$env:GOCMD_VER/gocmd-$env:GOCMD_VER-windows-amd64.zip ; tar zxvf gocmd.zip ; del gocmd.zip ; del gocmdv.txt
```

### Intel/AMD 32-bit

```cmd
curl -L -s -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt && set /p GOCMD_VER=<gocmdv.txt
curl -L -s -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/%GOCMD_VER%/gocmd-%GOCMD_VER%-windows-386.zip && tar zxvf gocmd.zip && del gocmd.zip gocmdv.txt
```

#### PowerShell

```powershell
curl -o gocmdv.txt https://raw.githubusercontent.com/cyverse/gocommands/main/VERSION.txt ; $env:GOCMD_VER = (Get-Content gocmdv.txt)
curl -o gocmd.zip https://github.com/cyverse/gocommands/releases/download/$env:GOCMD_VER/gocmd-$env:GOCMD_VER-windows-386.zip ; tar zxvf gocmd.zip ; del gocmd.zip ; del gocmdv.txt
```

## Manual Installation or Specific Versions  

If you need a specific release version or prefer to manually download the binaries, visit the GoCommands releases page:  

[GoCommands Releases](https://github.com/cyverse/gocommands/releases)

Here, you can browse and download any available version of GoCommands for your operating system.
