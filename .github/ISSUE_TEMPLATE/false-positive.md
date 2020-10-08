---
name: False positive
about: Report a false positive.
title: "[FALSE POSITIVE]: "
labels: false positive
assignees: mlevesquedion

---

## False positive report

Use this issue template to describe a false positive report produced by the analyzer. A false positive report is a report produced by the analyzer on code that you consider to be safe. For example, if the analyzer were to produce a report on the following piece of code, then that would be a false positive:

```go
func NothingWrongHere() {
    Sink(Safe{"not a secret"}) // we don't want a report here, since the value that reached the sink is not sensitive
}
```
(We are assuming that `Sink` has been configured as a sink and that `Safe` has *not* been configured as a source.)

**Describe the issue**
Please include a clear and concise description of what happened and why you think it is a false positive.

**To Reproduce**
Please make it as easy as possible for us to reproduce what you observed. If possible, provide the exact configuration and code on which the analyzer failed to produce a report. If the code cannot be shared, please provide a simplified example and confirm that it also yields the false positive report.

**Additional context**
Add any other context about the problem here.
