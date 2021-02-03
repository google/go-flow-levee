# Quickstart: How to configure and run the analyzer

The code for this example is available in the `go-flow-levee/guides/quickstart` directory.

## Context

Suppose you have the following piece of code:

```go
// quickstart.go
package quickstart

import "log"

type Authentication struct {
	Username string
	Password string
}

func authenticate(auth Authentication) (*AuthenticationResponse, error) {
	response, err := makeAuthenticationRequest(auth)
	if err != nil {
		log.Printf("unable to make authenticated request: incorrect authentication? %v\n", auth)
		return nil, err
	}
	return response, nil
}
```

This code is an instance of [CWE-522](https://cwe.mitre.org/data/definitions/522.html), "Insufficiently Protected Credentials".
The `Password` field on the `Authentication` struct contains credentials, which should not be allowed to reach logs.

This example may seem contrived, but the `go-flow-levee` analyzer has actually caught many similar cases in the wild.

## Configuration

Let's see how we can configure the analyzer to automatically detect this incorrect handling of credentials.

In order to do its job, the analyzer needs 2 pieces of information:
1. Which types hold sensitive data? (Sources)
2. Which functions should not be called with sensitive data? (Sinks)

Given this information, the analyzer will perform taint propagation to determine whether sensitive values can reach calls to functions that they shouldn't reach.

### Configuring a Source type

In taint propagation lingo, a value that contains data of interest is called a Source.
In the current example, a Source is a value that contains sensitive data, e.g. a password.  

There are 2 ways to define a Source type:
1. Define a field tag and apply it to each sensitive field
1. Describe a type using its name, the full path of its package, and the name of the sensitive field

#### Using field tags

In the analyzer's configuration, define the field tag:

```yaml
# analyzer_configuration.yaml
FieldTags:
  - Key: datapolicy
    Value: secret
```

In the code you want to analyze, add the tag to the sensitive field:

```go
// quickstart.go
type Authentication struct {
  Username string 
  Password string `datapolicy:"secret"`
}
```

If you do not wish to define your own field tag, you may use the built-in `levee:"source"` tag.

This method of configuration is recommended for its maintainability. 

#### Using type descriptions

In the analyzer's configuration, identify the sensitive data:

```yaml
# analyzer_configuration.yaml
Sources:
  - Package: github.com/google/go-flow-levee/guides/quickstart
    Type: Authentication
    Field: Password
```

This method of configuration does not require changes to the code, but you may need to change your
configuration more often. For example, if the type's named were shortened to `Auth`, you would need
to update the configuration to reflect that.

See [the documentation](../configuration/README.md) for further instructions on how to describe sources.

### Configuring a Sink

In taint propagation lingo, a function that should not be called with a tainted value is called a "Sink".
In the current example, a Sink is a function that could leak a sensitive value, e.g. by writing it to a log.

Configuring a sink is similar to configuring a source using a type description.
In the analyzer's configuration, identify the function:

```yaml
# analyzer_configuration.yaml
Sinks:
  - Package: log
    Method: Printf
```

See [the documentation](../configuration/README.md) for further instructions on how to describe sinks.

## Running the analyzer

Now that the analyzer knows what our Sources and Sinks are, we can run it to detect the issue in the example code.

### Install the analyzer

Use the following command to install the analyzer:

```shell
go get github.com/google/go-flow-levee/cmd/levee
```

### Run the analyzer

The most convenient way to run the analyzer is to use the `go vet` command.

You must provide `go vet` with three pieces of information:
1. What tool to run (in this case, the `levee` binary you just installed)
1. What configuration the analyzer should use
1. The list of packages to analyze

Use the following command to run the analyzer:
```shell
# in the go-flow-levee/guides/quickstart directory
go vet -vettool=$(which levee) -config=$(realpath analyzer_configuration.yaml) ./...
```

Running on the example code, you should see output similar to the following:

```
# github.com/google/go-flow-levee/guides/quickstart
./quickstart.go:14:13: a source has reached a sink
 source: ./quickstart.go:11:19
```

The analyzer detected the issue, and it produced a helpful report
indicating the locations of the source and sink in the code.

Let's fix the issue. Do we really need to be logging the `auth` struct? Maybe not:

```go
// quickstart.go
log.Printf("unable to make authenticated request: incorrect authentication?\n")
```

After making this change, the analyzer no longer produces a report. There are many ways to
address reports. The important thing is to modify the code so that the Source can no longer
reach the Sink. For example, you may wish to define a "Sanitizer" function that redacts the
`Password` from `Authentication` values, making them safe to log. See [the documentation](../configuration/README.md)
for more on how to configure sanitizers.

## Conclusion

In this example, we showed how the `go-flow-levee` analyzer can detect incorrect handling of
credentials. However, note that the analyzer can be used for any kind of taint propagation,
such as detecting code that is vulnerable to SQL injection.

Also note that when the analyzer does not produce any reports on a codebase, it does *not* mean
that the code base is "safe". There are inherent limitations to static analysis, so we recommend that
you also use a form of dynamic analysis, such as a sanitizing logger. Furthermore, you should verify
your configuration using simple examples like the one presented here to make sure that a lack of reports
is not caused by a configuration error.

Finally, in some cases the analyzer may produce reports that are in fact incorrect. Indeed, the analyzer
does not attempt to model your program's behavior completely, and its analysis is currently limited to one
function at a time. How to deal with "false positive" reports is described in [the documentation](../configuration/README.md).
