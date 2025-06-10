# Examples CLI Tool

This CLI tool is designed to extract and apply cluster class examples.

## Usage

From the `examples/` directory, configure the go workspace and download the dependencies:

```bash
go work use ./
go mod download
```

Run the program with:

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

## Running from tag or latest

You can run the examples CLI tool using `go run`. This method allows you to execute the tool directly from the module path without needing to clone the repository locally first.

Using a specific tag (`@<tag>`) is recommended for reproducible results, while `@latest` will always fetch the most recent version.

To run the latest version:

```bash
go run github.com/rancher/turtles/examples@latest
```

To run from a specific tag:

```bash
go run github.com/rancher/turtles/examples@<tag>
```

Make sure to replace `<tag>` with the desired tag name.

### Example: applying a specific cluster class from a specific tag

Apply examples from the `azure-aks` cluster class in the default namespace:

```bash
go run github.com/rancher/turtles/examples@<tag> azure-aks | kubectl apply -f -
```

Apply all `azure` example cluster classes in a custom namespace:

```bash
go run github.com/rancher/turtles/examples@<tag> -r azure | kubectl apply -f -n <namespace> -
```
