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
If neither a literal nor a regexp is provided, a wildcard matcher is assumed.
Providing both a literal and a regexp matcher is considered a misconfiguration and will error.

```yaml
Sources:
- Package: "full/package/path"
  # Neither Type nor TypeRE are specified - match all type names
  FieldRE: "Token|Password|Secret" 
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
  Value: source
```

Sinks and sanitizers are identified by package, method, and (if applicable) receiver name.
As with source configuration, these may be specified by either a provided string literal or regexp.
Use `Package`, `Receiver`, and `Method` to specify by string literal.
Use `PackageRE`, `ReceiverRE`, and `MethodRE` to specify by regexp.
If neither a literal nor a regexp is provided, a wildcard matcher is assumed.
Providing both a literal and a regexp matcher is considered a misconfiguration and will error.

```yaml
Sinks:
- PackageRE:  ".*/sinks(/v1)?"  # Regexp match a collection of packages 
  # Neither Receiver nor ReceiverRE is provided - match any (or no) receiver
  Method: "Sink"  # Match only functions named exactly "Sink"
Sanitizers:
- # Neither Package nor PackageRE is provided - match any package
  ReceiverRE: "^Safe"  # Match any receiver beginning with "Safe"
  Method: "sanitize"  # Match methods named exactly "sanitize"
```

To explicitly match an empty string, such as top-level functions without a receiver, explicitly configure an empty string matcher, e.g., `Receiver: ""`.

Taint propagation is performed automatically and does not need to be explicitly configured.

### Allowing panics on tainted values

By default, the `panic` builtin is considered a sink.
Indeed, if a tainted value reaches a call to `panic`,
it is likely that this tainted value will escape the program's memory.
The analyzer makes no attempt to determine whether a `panic` can actually can occur,
nor does it try to determine whether each call to `panic` will be caught by a call to `recover`.

If you do not wish `panic` to be considered a sink, add the following line to your configuration:

```yaml
AllowPanicOnTaintedValues: true
```

### Restricting analysis scope

Functions can be explicitly excluded from analysis using string literals or regexps,
constructed similarly to those used to identify sanitizers and sinks:
```yaml
Exclude:
- Package: "myproject/mypackage"
  MethodRE: "^my.*"
```

The above will match any function beginning with "my" in the `myproject/mypackage` package.
Since no receiver matcher was provided, it will match any method beginning with "my" bound to any (or no) receiver.

As just two examples, this may be used to avoid analyzing test code, or to suppress "false positive" reports.

### Suppressing false positives

The analyzer may produce reports on pieces of code that are actually safe. If you run across such a "false positive" report, you may suppress it by adding a comment above the line where the taint reached the `sink`:

```go
// levee.DoNotReport
mysinks.SinkF("here's a value: %v", safeValue)
```

You may include a justification in the suppression comment, e.g.:

```go
// levee.DoNotReport: I have verified that "safeValue" cannot actually contain sensitive data (mlevesquedion, Feb 05 2021).
```

In fact, as long as the exact string `levee.DoNotReport` appears at the beginning of a line in a comment above the line where a report would be produced, the report will be suppressed. In most cases, you may also be able to suppress a report by placing the comment on the line itself, e.g.:

```go
mysinks.SinkF("here's a value: %v", safeValue) // levee.DoNotReport
```

Finally, note that you can't suppress a report for a specific argument, so the following will not work:

```go
mysinks.Sinkln(safeValue, // levee.DoNotReport
  otherValue)
```

A few things to keep in mind when using suppression:
* Before suppressing, you should validate that a tainted value really can't reach a sink (i.e., you are really suppressing a _false_ positive).
* You should periodically reexamine your suppressions to make sure that they are still accurate. If you suppress a report, but later on the code changes such that the report on a given line would actually be a _true_ positive, the analyzer won't tell you about it.

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
