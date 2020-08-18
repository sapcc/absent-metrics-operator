# Absent Metrics Operator

[![GitHub Release](https://img.shields.io/github/v/release/sapcc/absent-metrics-operator)](https://github.com/sapcc/absent-metrics-operator/releases/latest)
[![GitHub Workflow Status](https://img.shields.io/github/workflow/status/sapcc/absent-metrics-operator/Build%20and%20Test)](https://github.com/sapcc/absent-metrics-operator/actions?query=workflow%3A%22Build+and+Test%22)
[![Coveralls github](https://img.shields.io/coveralls/github/sapcc/absent-metrics-operator)](https://coveralls.io/github/sapcc/absent-metrics-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/sapcc/absent-metrics-operator)](https://goreportcard.com/report/github.com/sapcc/absent-metrics-operator)
[![Docker Pulls](https://img.shields.io/docker/pulls/sapcc/absent-metrics-operator)](https://hub.docker.com/r/sapcc/absent-metrics-operator)

> Project status: **alpha**. The API and user facing objects may change.

In this document:

- [Overview](#overview)
- [Motivation](#motivation)
- [Usage](#usage)
  - [Metrics](#metrics)
- [Absent metric alert definition](#absent-metric-alert-definition)
  - [Template](#template)
  - [Labels](#labels)
    - [Defaults](#defaults)
    - [Carry over from original alert rule](#carry-over-from-original-alert-rule)
    - [Tier and service](#tier-and-service)

In other documents:

- [Operator's Playbook](./doc/playbook.md)

## Overview

The absent metrics operator is a companion operator for the [Prometheus
Operator](https://github.com/prometheus-operator/prometheus-operator).

The operator monitors all the `PrometheusRule` resources deployed across a
Kubernetes cluster and creates corresponding _absent metric alert rules_ for
the alert rules defined in those resources.

An absent metric alert rule alerts on the absence of a metric.

## Motivation

Consider the following alert rule definition:

```yaml
alert: ImportantAlert
expr: foo_bar > 0
for: 5m
labels:
  tier: network
  service: foo
  severity: critical
annotations:
  summary: Data center is on fire!
```

This alert would never trigger if the metric `foo_bar` does not exist in
Prometheus.

This can be avoided by using the `absent()` function with the `or` operator so
the alert rule expression becomes:

```
absent(foo_bar) or foo_bar > 0
```

This gets tedious if you have hundreds of alerts deployed across the cluster.
There is also the element of human error, e.g. typo or forgetting to include
the `absent` function in the alert expression.

This problem is resolved by the absent metrics operator as it automatically
creates the corresponding absent metric alerts for your alert definitions.

The operator would generate the following absent metric alert for the above
example:

```yaml
alert: AbsentFooBar
expr: absent(foo_bar)
for: 10m
labels:
  tier: network
  service: foo
  severity: info
annotations:
  summary: missing foo_bar
  description: The metric 'foo_bar' is missing. 'ImportantAlert' alert using it may not fire as intended.
```

## Usage

We provide pre-compiled binaries and container images. See the latest
[release](https://github.com/sapcc/absent-metrics-operator/releases/latest).

Alternatively, you can build with `make`, install with `make install`, `go get`, or
`docker build`.

For usage instructions:

```
$ absent-metrics-operator --help
```

You can disable the the operator for a specific `PrometheusRule` or a specific alert definition, refer to the [operator's playbook](./doc/playbook.md) for more info.

### Metrics

Metrics are exposed at port `9659`. This port has been
[allocated](https://github.com/prometheus/prometheus/wiki/Default-port-allocations)
for the operator.

| Metric                                              | Labels                                            |
| --------------------------------------------------- | ------------------------------------------------- |
| `absent_metrics_operator_successful_reconcile_time` | `prometheusrule_namespace`, `prometheusrule_name` |

## Absent metric alert definition

The absent metric alerts are defined in a separate `PrometheusRule` resource
that is managed by the operator. They are aggregated first by namespace and
then by the Prometheus server.

For example, if a namespace has alert rules defined across several
`PrometheusRule` resources for the Prometheus servers called `OpenStack` and
`Infra`. The absent metric alerts for this namespace would be aggregated in two
new `PrometheusRule` resources called:

- `openstack-absent-metric-alert-rules`
- `infra-absent-metric-alert-rules`

### Template

The absent metric alert rule has the following template:

```yaml
alert: $name
expr: absent($metric)
for: 10m
labels:
  tier: $tier
  service: $service
  severity: info
annotations:
  summary: missing $metric
  description: The metric '$metric' is missing. '$alert-name' alert using it may not fire as intended.
```

Consider the metric `limes_successful_scrapes:rate5m` with tier `os` and
service `limes`.

Then the alert name would be `AbsentOsLimesSuccessfulScrapesRate5m`.

### Labels

#### Defaults

The following labels are always present on every absent metric alert rule:

- `severity` is alway `info`.
- `playbook` provides a [link](./doc/playbook.md) to documentation that can be
  referenced on how to deal with an absent metric alert.

#### Carry over from original alert rule

You can specify which labels to carry over from the original alert rule by
specifying a comma-separated list of labels to the `--keep-labels` flag. The
default value for this flag is `service,tier`.

#### Tier and service

`tier` and `service` labels are a special case they are carried over from the
original alert rule unless those labels use templating (i.e. use `$labels`), in
which case the default `tier` and `service` values will be used.

The operator determines a default `tier` and `service` for a specific
Prometheus server in a namespace by traversing through all the alert rule
definitions for that Prometheus server in that namespace. It chooses the most
common `tier` and `service` label combination that is used across those alerts
as the default values.

The value of these labels are also for used (if enabled with `keep-labels`) in
the name for the absent metric alert. See [template](#Template).
