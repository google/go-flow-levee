---
name: False positive
about: This template is used to describe false positive reports.
title: "[FALSE POSITIVE]: "
labels: false positive
assignees: mlevesquedion

---

## False positive report

Use this issue template to describe a false positive report produced by the analyzer. A false positive report is a report produced by the analyzer on code that you consider to be safe. For example, if the analyzer were to produce a report on the following piece of code, then that would be a false positive:

```go
func NothingWrongHere() {
    Sink(nil) // we don't want a report here, since the value that reached the sink is not sensitive
}
```

Please make it as easy as possible for us to reproduce what you observed. If possible, provide the exact configuration and code on which the analyzer produced a report. If you cannot do that (e.g., the code is closed source), please provide a simplified example and confirm that it also yields the false positive report.
