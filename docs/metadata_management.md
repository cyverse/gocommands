# Metadata Management

GoCommands provides features to manage metadata for data objects, collections, resources, and users in the Data Store using the `lsmeta`, `addmeta`, and `rmmeta` commands.

**Metadata Components:**

- **Name (Attribute):** The name of information
- **Value:** The actual information or data
- **Unit (Optional):** Specifies the unit of measurement, if applicable

## :material-tag-edit-outline: List Metadata of Data Objects, Collections, Resources, or Users

```sh
gocmd lsmeta [flags] <irods-object>...
```

### iRODS Objects 

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| `data object` or `collection` | `-P` | Add metadata to a data object or collection |
| `resource` | `-R` | Add metadata to a resource |
| `user` | `-U` | Add metadata to a user |

### Example Usage

1. **List metadata of a data object:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/file.txt
    ```

2. **List metadata of multiple data objects:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/file1.txt /myZone/home/myUser/file2.txt
    ```

3. **List metadata of a collection:**
    ```sh
    gocmd lsmeta -P /myZone/home/myUser/dir
    ```

4. **List metadata of a resource:**
    ```sh
    gocmd lsmeta -R myResc
    ```

5. **List metadata of a user:**
    ```sh
    gocmd lsmeta -U myUser
    ```

## :material-tag-edit-outline: Add Metadata to Data Objects, Collections, Resources, or Users

```sh
gocmd addmeta [flags] <irods-object> <metadata-name> <metadata-value> [metadata-unit]
```

### iRODS Objects 

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| `data object` or `collection` | `-P` | Add metadata to a data object or collection |
| `resource` | `-R` | Add metadata to a resource |
| `user` | `-U` | Add metadata to a user |

### Example Usage

1. **Add metadata to a data object:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/file.txt meta_name meta_value
    ```

1. **Add metadata to a data object with metadata-unit:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/file.txt meta_name meta_value meta_unit
    ```

3. **Add metadata to a collection:**
    ```sh
    gocmd addmeta -P /myZone/home/myUser/dir meta_name meta_value
    ```

4. **Add metadata to a resource:**
    ```sh
    gocmd addmeta -R myResc meta_name meta_value
    ```

5. **Add metadata to a user:**
    ```sh
    gocmd addmeta -U myUser meta_name meta_value
    ```

## :material-tag-edit-outline: Remove Metadata from Data Objects, Collections, Resources, or Users

```sh
gocmd rmmeta [flags] <irods-object> <metadata-ID-or-name>
```

**Note:** The `metadata-ID` is a numeric identifier for the metadata. It can be obtained from the output of the `lsmeta` command.

---
Answer from Perplexity: pplx.ai/share

### iRODS Objects 

| iROD Object | Flag | Description |
|-------------|-------------|--------|
| `data object` or `collection` | `-P` | Remove metadata from a data object or collection |
| `resource` | `-R` | Remove metadata from a resource |
| `user` | `-U` | Remove metadata from a user |

### Example Usage

1. **Remove metadata from a data object by name:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/file.txt meta_name
    ```

2. **Remove metadata from a data object by ID:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/file.txt 979206950
    ```

3. **Remove metadata from a collection:**
    ```sh
    gocmd rmmeta -P /myZone/home/myUser/dir meta_name
    ```

4. **Remove metadata from a resource:**
    ```sh
    gocmd rmmeta -R myResc meta_name
    ```

5. **Remove metadata from a user:**
    ```sh
    gocmd rmmeta -U myUser meta_name
    ```
