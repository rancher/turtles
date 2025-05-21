# Examples CLI Tool

This CLI tool is designed to extract and apply cluster class examples.

## Usage

```bash
go run main.go <search_key>
```

`<search_key>`: The key to search for in the available cluster classes. In case of an incorrect `ClusterClass` key used, the closest matching key will be suggested.

## Flags

*   `-l`, `--list`: List available cluster class names from examples.
*   `-r`, `--regex`: ClusterClass search regex.

## Examples

### List available cluster classes

```bash
go run main.go -l
```

Example output:

```text
Available classes: [azure-aks-example azure-example azure-rke2-example]
```

### Search for a specific cluster class

```bash
go run main.go azure-aks
```

### Search for cluster classes using a regex

```bash
go run main.go -r "azure"
```

Note: Regex search can return multiple examples.

To apply the extracted examples, you can use the following command:

```bash
go run main.go <search_key> | kubectl apply -f -
```
