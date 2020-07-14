# Configuring and running Go Flow Levee

***Many terms and related configuration elements are experimental and likely to change in the future.***

## Install or build go-flow-levee

Acquire the binary via `go install github.com/google/go-flow-levee/cmd/levee`.
Alternatively, build this repository from its root directory via `go build -o /install/destination/path ./cmd/levee`.

## Configuration

For design details concerning value and instruction classification, see [/design](../design/README.md).

Configuration is provided to `go-flow-levee` via JSON.

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

Sinks and sanitizers are identified via regexp according to package and method name.

```json
{
  "Sinks": [
    {
      "PackageRE": "<package path regexp>",
      "MethodRE": "<method name regexp>"
    }
  ],
  "Sanitizers": [
    {
      "PackageRE": "<package path regexp>",
      "MethodRE": "<method name regexp>"
    }
  ]
}
```

We separate propagators into the following types:

A *transforming propagator* produces a tainted value which may contain source data.
For instance, a struct's `String()` method or a serializable's `Marshal()` method are transforming propagators.
The value returned by either can contain source data without being identifiable as the specified source's type.
Transforming propagators are identified via regexp according to package and function name.

A *field propagator* is similar to a transforming propagator,
but it accepts a fully-qualified receiver name rather than a package name.
Field propagators should include getter methods.

An *argument propagator* is a propagator which taints an input argument rather than a return value.
This is done, e.g., by buffer writers such as `fmt.Fprintf`, though could conceivably apply to any function that takes a source and any other reference argument.
At time of writing, argument propagators only applies to the first argument of a method, expecting a pattern similar to buffer writers.
Configuration currently only allows for one argument propagator to be specified.
Argument propagators are identified via regexp matching the fully-qualified type name.

```json
{
  "TransformingPropagators": [
    {
      "PackageRE": "<package path regexp>",
      "MethodRE": "<method name regexp>"
    }
  ],
  "FieldPropagators": [
    {
      "Receiver": "<fully qualified type path regexp>",
      "AccessorRE": "<method name regexp>"
    }
  ],
  "PropagatorArgs": {
    "ArgumentTypeRE": "^io\\.(?:Writer|ReadWriter|WriteCloser|ReadWriteCloser)$"
  }
}
```

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
