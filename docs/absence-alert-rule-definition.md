# Absence alert rule definition

> This document assumes that you have already read and understood the [general
> README](../README.md). If not, start reading there.

This document describes how _absence alert rules_ are defined.

## Aggregation

The _absence alert rules_ are defined in a separate `PrometheusRule` resource that is
managed by the operator. They are aggregated first by namespace and then by the Prometheus
server.

For example, if a namespace has alert rules defined across several `PrometheusRule`
resources for the Prometheus servers called `OpenStack` and `Infra`. The _absent alert
rules_ for this namespace would be aggregated in two new `PrometheusRule` resources
called:

- `openstack-absent-metric-alert-rules`
- `infra-absent-metric-alert-rules`

## Rule Template

The _absence alert rule_ has the following template:

```yaml
alert: $name
expr: absent($metric)
for: 10m
labels:
  context: absent-metrics
  severity: info
  support_group: $support_group
  service: $service
annotations:
  summary: missing $metric
  description: The metric '$metric' is missing. '$alert-name' alert using it may not fire as intended.
```

Consider the an alert rule that uses a metric called `limes_successful_scrapes:rate5m`
with support group `containers` and service `limes` labels. The name of the corresponding
_absence alert rule_ would be `AbsentContainersLimesSuccessfulScrapesRate5m`.

The values of `support_group` and `service` labels are only included in the name if the
labels are specified in the `--keep-labels` flag.

The description also includes a [link](./doc/playbook.md) to the playbook for operators
that can be referenced on how to deal with _absence alert rules_.

## Labels

Labels which are specified with the `--keep-labels` flag will be retained from the
original alert rule and will be defined on the corresponding _absence alert rule_ as is.

The `support_group` and `service` labels are a special case, they have some custom behavior which is
defined in the [playbook for operators](./playbook.md#support-group-and-service-labels).

### Defaults

The following labels are always present on all _absence alert rules_:

- `severity: info`
- `context: absent-metrics`
