# Configuring and running Go Flow Levee

***Many terms and related configuration elements are experimental and likely to change in the future.***

## Install or build go-flow-levee

Acquire the binary via `go install github.com/google/go-flow-levee/cmd/levee`.
Alternatively, build this repository from its root directory via `go build -o /install/destination/path ./cmd/levee`.

## Configuration

For design details concerning value and instruction classification, see [/design](../design/README.md).

Configuration is provided to `go-flow-levee` via YAML.

Sources are identified by package, type, and field names.
You may specify these with a combination of string literals or regexp.
Use `Package`, `Type`, and `Field` to specify by string literal.
Use `PackageRE`, `TypeRE`, and `FieldRE` to specify by regexp.
You may use a combination as needed.

```yaml
Sources:
# Specify using string literals
- Package: "literal/package/path"
  Type: "typeName"
  Field: "fieldName"
# Specify using regexp
- PackageRE: "<package path regexp>"
  TypeRE: "<type name regexp>"
  FieldRE: "<field name regexp>"
# Specify using string literals and regexp
- Package: "literal/package/path"
  TypeRE: ".*"
  Field: "fieldName"
```

Sources may also be identified via field tags:
```go
type Example struct {
	fieldName fieldType `levee:"source"` // this field will be considered a Source
}
```

The tag `levee:"source"` is built-in. Additional tags may be identified via explicit string literals (not regexps). The following example shows how the `levee:"source"` tag could be defined if it weren't built-in:
```yaml
FieldTags:
- Key: levee
  Val: source
```

Sinks and sanitizers are identified by package, method, and (optional) receiver name.
Again, these may be specified by either a provided string literal or regexp.

```yaml
Sinks:
- Package:  "literal/package/path" 
  ReceiverRE: <type name regexp>
  MethodRE: <method name regexp>
Sanitizers:
- PackageRE: <package path regexp>
  ReceiverRE: <type name regexp>
  Method: "mySanitizer"
```

Taint propagation is performed automatically and does not need to be explicitly configured.

### Important Note

Configuration of the above matchers does not require listing of all arguments.
E.g., the following is a valid source configuration:
```yaml
Sources:
- Package: "literal/package/path"
  Type: "MySourceType"
```

Neither `Field` nor `FieldRE` have been provided.
In this case, we match all fields of `MySourceType`, assuming a wildcard matcher for `FieldRE`.
Similar behavior exists in for all attributes, e.g. providing neither `Type` nor `TypeRE` will match all type names.

To explicitly match an empty string, such as top-level functions without a receiver, explicitly define the attribute by the corresponding string literal, `Receiver: ""`, or an anchored regexp, `ReceiverRE: "^$"`.

### Restricting analysis scope

Functions can be explicitly excluded from analysis using string literals or regexps,
constructed similarly to those used to identify sanitizers and sinks:
```yaml
Exclude:
- Package: "myproject/mypackage"
  MethodRE: "^my.*"
```

The above will match the function beginning with "my" in the `myproject/mypackage` package.
Since no receiver matcher was provided, it will match any method beginning with "my" bound to any receiver.

As just two examples, this may be used to avoid analyzing test code, or to suppress "false positive" reports.

### Example configuration

The following configuration could be used to identify possible instances of credential logging in Kubernetes.

[example-config.yaml](example-config.yaml)

## Execution

The `go-flow-levee` binary can be run directly, or via `go vet -vettool /path/to/levee`.
In either case, a `-config /path/to/configuration` will be required.

Analysis is executed per package.
This can often be achieved with Go's `...` package expansion, e.g. 
```bash
go vet -vettool /path/to/levee -config /path/to/config -- code/to/analyze/root/...
```

For an end-to-end example, refer to [example.sh](example.sh).
