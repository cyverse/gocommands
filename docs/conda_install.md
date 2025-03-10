# Installing GoCommands via Conda

`GoCommands` can be installed via `conda` if you are using Linux or Mac OS. Unfortunately, Windows system is not yet supported. Please follow instructions below to install.

## Add conda-forge Channel

Add `conda-forge` channel to `conda`. This is required because `GoCommands` is added to `conda-forge` channel.

```bash
conda config --add channels conda-forge
conda config --set channel_priority strict
```

## Install GoCommands

Install `GoCommands` with `conda`.

```bash
conda install gocommands
```

This will install the latest version of GoCommands available in the conda-forge repository.