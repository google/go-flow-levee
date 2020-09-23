# Configuring and running Go Flow Levee

***Many terms and related configuration elements are experimental and likely to change in the future.***

## Install or build go-flow-levee

Acquire the binary via `go install github.com/google/go-flow-levee/cmd/levee`.
Alternatively, build this repository from its root directory via `go build -o /install/destination/path ./cmd/levee`.

## Configuration

For design details concerning value and instruction classification, see [/design](../design/README.md).

Configuration is provided to `go-flow-levee` via JSON.

Objects of interest are identified primarily via regexp. An empty regexp will match any string.

Sources are identified via regexp according to package, type, and field names.
```json
{
  "Sources": [
    {
      "PackageRE": "<package path regexp>",
      "TypeRE": "<type name regexp>",
      "FieldRE": "<field name regexp>"
    }
  ]
}
```

Sinks and sanitizers are identified via regexp according to package, method, and (optional) receiver name.

```json
{
  "Sinks": [
    {
      "PackageRE": "<package path regexp>",
      "ReceiverRE": "<type name regexp>",
      "MethodRE": "<method name regexp>"
    }
  ],
  "Sanitizers": [
    {
      "PackageRE": "<package path regexp>",
      "ReceiverRE": "<type name regexp>",
      "MethodRE": "<method name regexp>"
    }
  ]
}
```

Taint propagation is performed automatically and does not need to be explicitly configured.

For matchers that accept a `ReceiverRE` regexp matcher, an unspecified string will match any (or no) receiver.
To match only methods without any receiver (i.e., a top-level function), use the matcher `^$` to match an empty-string receiver name.

### Restricting analysis scope

Functions can be explicitly excluded from analysis using regexps:
```json
{
  "Exclude": [
    {
      "PathRE": "^mypackage/myfunction$"
    }
  ]
}
```

The above will match the function `myfunction` from the `mypackage` package. It will also match a method named `myfunction` in the same package.

As just two examples, this may be used to avoid analyzing test code, or to suppress "false positive" reports.

### Example configuration

The following configuration could be used to identify possible instances of credential logging in Kubernetes.

[example-config.json](example-config.json)

## Execution

The `go-flow-levee` binary can be run directly, or via `go vet -vettool /path/to/levee`.
In either case, a `-config /path/to/configuration` will be required.

Analysis is executed per package.
This can often be achieved with Go's `...` package expansion, e.g. 
```bash
go vet -vettool /path/to/levee -config /path/to/config -- code/to/analyze/root/...
```

For an end-to-end example, refer to [example.sh](example.sh).
