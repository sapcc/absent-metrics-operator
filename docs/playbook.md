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

## Support group and service labels

`support_group` and `service` labels are a special case. We (SAP Converged Cloud) use them for
routing alert notifications to different channels.

These labels are defined using different strategies in the following order
(highest to lowest priority):

1. Alert rule labels: if the alert rule has the `support_group` **OR** `service` label and the
   label doesn't use templating (e.g. `$labels.some_label`) then carry over that label as
   is.
2. K8s object level labels: If the `support_group` **OR** `service` labels are defined at the
   object (i.e. `PrometheusRule`) level then use their values.
3. Most common `support_group`/`service` combination: find a default value for the
   `support_group` and `service` labels by traversing through all the alert rules defined
   in the `PrometheusRule` object. The `support_group` **AND** `service` label combination
   that is the most common amongst all those alerts will be used as the default.
4. Most common `support_group`/`service` combination across the namespace: traverse
   through all the alert rule definitions for the concerning Prometheus server in the
   concerning namespace. The `support_group` **AND** `service` label combination that is
   the most common amongst all those alerts will be used as the default.

If all of the above strategies fail, i.e. a value for `support_group` and `service` cannot
be determined, then the _absence alert rules_ won't have these labels.

**Tip**: add `ccloud/support-group` and `ccloud/service` labels to your `PrometheusRule`
objects. These values will be used as defaults in case your alert rule definitions are
missing these labels or if templating is used. This will ensure that the alert
notifications for _absence alerts_ will be routed to the correct channels.
