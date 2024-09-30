# Absence alert rule definition

> [!NOTE]
> This document assumes that you have already read and understood the [general README](../README.md). If not, start reading there.

This document describes how _absence alert rules_ are defined.

## Aggregation

The _absence alert rules_ are defined in a separate `PrometheusRule` resource that is
managed by the operator. They are aggregated first by namespace and then using the
template provided in the `--prom-rule-name` flag.

The default template is:

```go
{{ if index .metadata.labels "thanos-ruler" }}{{ index .metadata.labels "thanos-ruler" }}{{ else }}{{ index .metadata.labels "prometheus" }}{{ end }}
```

this means that the absence alert rules will be aggregated in a namespace by the `thanos-ruler` label if it exists otherwise the `prometheus` label.

For example, if a namespace has alert rules defined across several `PrometheusRule`
resources for the Prometheus servers called `OpenStack` and `Infra`. The _absent alert
rules_ for this namespace would be aggregated in two new `PrometheusRule` resources
called:

- `openstack-absent-metric-alert-rules`
- `infra-absent-metric-alert-rules`

### Examples
Here are some example templates that you could use for aggregation:

- 1:1 AbsencePrometheusRule creation, i.e. for each `PrometheusRule` object, create a corresponding `PrometheusRule` object that holds the absence alerts
  ```go
  {{ .metadata.name }}
  ```
- One AbsencePrometheusRule per namespace — this will create one `PrometheusRule` object which will hold all the absence alert rules for that namespace
  ```go
  {{ .metadata.namespace }}
  ```

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

The description also includes a [link](./docs/playbook.md) to the playbook for operators
that can be referenced on how to deal with _absence alert rules_.

## Labels

The following labels are always present on all _absence alert rules_:

- `severity: info`
- `context: absent-metrics`

Additionally, labels which are specified with the `--keep-labels` flag will be copied over verbatim from the original alert rule to the corresponding _absence alert rule_.
