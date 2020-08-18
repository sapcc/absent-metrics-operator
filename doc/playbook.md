# Operator's Playbook

This document assumes that you have already read and understood the [general
README](../README.md). If not, start reading there.

### Disable for specific alerts

You can disable the operator for a specific `PrometheusRule` resource by adding
the following label to it:

```yaml
absent-metrics-operator/disable: "true"
```

If you want to disable the operator for only a specific alert rule instead of
all the alerts in a `PrometheusRule`, you can add the `no_alert_on_absence`
label to the alert rule. For example:

```yaml
alert: ImportantAlert
expr: foo_bar > 0
for: 5m
labels:
  no_alert_on_absence: "true"
  ...
```

**Note**: make sure that you use `"true"` and not `true`.

#### Caveat

If you disable the operator for a specific alert or a specific
`PrometheusRule`, however there are other alerts or `PrometheusRules` which
have alert definitions that use the same metric(s) then the absent metric
alerts for those metric(s) will be created regardless.
