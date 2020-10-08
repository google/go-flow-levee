---
name: False negative
about: This template is used to describe false negative reports.
title: "[FALSE NEGATIVE]: "
labels: false negative
assignees: ''

---

## False negative report

Use this issue template to describe a situation where the analyzer failed to recognize that a piece of unsafe code is unsafe. For example, if the analyzer did not produce a report on the following piece of code, then that would be a false negative:

```go
func Oops() {
    Sink(Source{"sensitive data"})
}
```
(We are assuming that `Source` has been configured as a source and `Sink` has been configured as a sink.)

Please make it as easy as possible for us to reproduce what you observed. If possible, provide the exact configuration and code on which the analyzer failed to produce a report. If the code cannot be shared, please provide a simplified example and confirm that it also contains the false negative.
