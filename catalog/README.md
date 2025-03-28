# Gocommands Configuration Catalog  

The *Gocommands Configuration Catalog* provides default iRODS configurations for iRODS hosts.  

Each configuration follows the iCommands Configuration File format in JSON (`~/.irods/irods_environment.json`).  

When running `gocmd init`, users can enter their `hostname` at the prompt to load the corresponding configuration, which then serves as the default. This catalog simplifies the setup of PAM/SSL authentication and other configurations.  

To add a new host, modify the `catalog.json` file and submit a Pull Request (PR) on GitHub.
