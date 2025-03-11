# File Encryption with GoCommands

GoCommands provides a secure file encryption feature to protect confidential data on iRODS. It encrypts both filenames and file content using the `AES256-CTL` algorithm before uploading files and decrypts them after downloading. By default, it uses the `RSA + AES256-CTL` algorithm with your SSH public key (`$HOME/.ssh/id_rsa.pub`) and private key (`$HOME/.ssh/id_rsa`).

The `put`, `get`, and `ls` commands support encryption and decryption.

---

## Uploading Files with Encryption

To upload a file (`file1.txt`) with encryption:
```
gocmd put --encrypt file1.txt target_dir
```

### Custom SSH Public Key
Specify a custom SSH public key file with:
```
gocmd put --encrypt --encrypt_pub_key id_rsa.pub file1.txt target_dir
```

After uploading, the file will be renamed with the `.rsaaesctr.enc` extension.

---

## Listing Encrypted Directories

To list a directory containing encrypted files and display original filenames:
```
gocmd ls --decrypt target_dir
```

### Custom SSH Private Key for Listing
Specify a custom SSH private key file with:
```
gocmd ls --decrypt --decrypt_priv_key id_rsa target_dir
```

---

## Downloading Encrypted Files

To download and decrypt an encrypted file:
```
gocmd get --decrypt target_dir/XXXXXXXXXXXXXXXXXXXXXXXXX.rsaaesctr.enc
```

### Custom SSH Private Key for Decryption
Specify a custom SSH private key file with:
```
gocmd get --decrypt --decrypt_priv_key id_rsa XXXXXXXXXXXXXXXXXXXXXXXXX.rsaaesctr.enc
```

---

## Available Flags for Encryption and Decryption

### Flags for `put` (Uploading Files)
| Flag                       | Description                                   | Default Value                     |
|----------------------------|-----------------------------------------------|-----------------------------------|
| `--encrypt_key` string      | Encryption key for `'winscp'`/`'pgp'` modes. | *None*                            |
| `--encrypt_mode` string    | Encryption mode (`'winscp'`, `'pgp'`, `'ssh'`). | `"ssh"`                         |
| `--encrypt_pub_key` string  | Public key for `'ssh'` mode.                  | `/home/iychoi/.ssh/id_rsa.pub`    |
| `--encrypt_temp` string    |  Temp directory for encryption.                | `"/tmp"`                          |

### Flags for `get` (Downloading Files)
| Flag                       | Description                                   | Default Value                     |
|----------------------------|-----------------------------------------------|-----------------------------------|
| `--decrypt`                | Enables decryption.                          | `true`                            |
| `--decrypt_key` string     | Decryption key for `'winscp'`/`'pgp'`.       | *None*                            |
| `--decrypt_priv_key` string | Private key for `'ssh'`.                      | `/home/iychoi/.ssh/id_rsa`        |
| `--decrypt_temp` string     | Temp directory for decryption.               | `"/tmp"`                          |

### Flags for `ls` (Listing Directories)
| Flag                       | Description                                   | Default Value                     |
|----------------------------|-----------------------------------------------|-----------------------------------|
| `--decrypt`                | Enables decryption of filenames.             | `true`                            |
| `--decrypt_key` string     | Decryption key for `'winscp'`/`'pgp'`.       | *None*                            |
| `--decrypt_priv_key` string | Private key for `'ssh'`.                      | `/home/iychoi/.ssh/id_rsa`        |
| `--decrypt_temp` string    | Temp directory for decryption.               | `"/tmp"`                          |

---

## Encryption Modes

- **SSH Mode (`ssh`)**: Uses RSA + AES256-CTL with SSH keys.
- **WinSCP Mode (`winscp`)**: Compatible with WinSCP.
- **PGP Mode (`pgp`)**: Compatible with PGP. Encrypts only file content.
