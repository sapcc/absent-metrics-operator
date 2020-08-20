# Operator's Playbook

In this document:

- [Disable for specific alerts](#disable-for-specific-alerts)
  - [Caveat](#caveat)
- [Tier and service labels](#tier-and-service-labels)

This document assumes that you have already read and understood the [general
README](../README.md). If not, start reading there.

## Disable for specific alerts

You can disable the operator for a specific `PrometheusRule` resource by adding
the following label to it:

```yaml
absent-metrics-operator/disable: "true"
```

If you want to disable the operator for only a specific alert rule instead of
all the alerts in a `PrometheusRule` then you can add the `no_alert_on_absence`
label to a specific alert rule. For example:

```yaml
alert: ImportantAlert
expr: foo_bar > 0
for: 5m
labels:
  no_alert_on_absence: "true"
  ...
```

**Note**: make sure that you use `"true"` and not `true`.

### Caveat

If you disable the operator for a specific alert or a specific
`PrometheusRule` but there are other alerts or `PrometheusRules` which
have alert definitions that use the same metrics then the absent metric
alerts for those metrics will be created regardless.

## Tier and service labels

`tier` and `service` labels are a special case. We (SAP CCloudEE) use them for
posting alert notifications to different Slack channels.

These labels are defined using different strategies in the following order
(highest to lowest priority):

1. If the alert rule has the `tier` **OR** `service` label and the label
   doesn't use templating (e.g. `$labels.some_label`) then carry over that
   label as is.
2. If the `tier` **OR** `service` labels are defined at the resource (i.e.
   `PrometheusRule`) level then use their values.
3. Find a default value for the `tier` and `service` labels by traversing
   through all the alert rule definitions for a specific Prometheus server in a
   specific namespace. The `tier` **AND** `service` label combination that is
   the most common amongst all those alerts will be used as the default.

If all of the above strategies fail, i.e. a value for `tier` and `service`
cannot be determined, then your absent metric alerts won't have these labels.

**TIP**: add default `tier` and `service` labels to your `PrometheusRule`. This
way if your alert is missing these labels or uses templating, the corresponding
absent metric alert will use these defaults and you'll ensure that your absent
alert notifications always land in the appropriate Slack channel.
