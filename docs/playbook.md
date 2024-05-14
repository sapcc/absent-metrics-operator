# Playbook for Operators

> [!NOTE]
> This document assumes that you have already read and understood the [general README](../README.md). If not, start reading there.

This document contains instruction on how to deal with _absence alerts_.

## Disable the operator

> [!IMPORTANT]
> Make sure that you use `"true"` (string) and not `true` (boolean) as the value for labels below.

### Specific alert rule

If you want to disable the operator for only a specific alert rule then you can add the
`no_alert_on_absence` label to a specific alert rule.

Example:

```yaml
alert: ImportantAlert
expr: foo_bar > 0
for: 5m
labels:
  no_alert_on_absence: "true"
  ...
```

### Entire `PrometheusRule`

You can disable the operator for a specific `PrometheusRule` resource by adding the
following label to it:

```yaml
absent-metrics-operator/disable: "true"
```

### Caveat

If you disable the operator for a specific alert or a specific
`PrometheusRule` resource but there are other alerts or `PrometheusRule` resources which
have alert definitions that use the same metrics then the _absent alert
rules_ for those metrics will be created regardless.

For example, considering the following rule definitions:

```yaml
- alert: ImportantAlert
  expr: foo_bar > 0
  for: 5m
  labels:
    no_alert_on_absence: "true"
    ...

- alert: ImportantServiceAlert
  expr: max(foo_bar) BY (service, region) > 0
  for: 5m
  labels:
    ...
```

An _absence alert rule_ for the `foo_bar` metric will be created because it is used in
`ImportantServiceAlert` even though `ImportantAlert` specifies the `no_alert_on_absence`
label.
