# Absent Metrics Operator

[![GitHub Release](https://img.shields.io/github/v/release/sapcc/absent-metrics-operator)](https://github.com/sapcc/absent-metrics-operator/releases/latest)
[![Build and Test](https://github.com/sapcc/absent-metrics-operator/workflows/Build%20and%20Test/badge.svg)](https://github.com/sapcc/absent-metrics-operator/actions?query=workflow%3A%22Build+and+Test%22)
[![Coverage Status](https://coveralls.io/repos/github/sapcc/absent-metrics-operator/badge.svg?branch=master)](https://coveralls.io/github/sapcc/absent-metrics-operator?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/sapcc/absent-metrics-operator)](https://goreportcard.com/report/github.com/sapcc/absent-metrics-operator)
[![Docker Pulls](https://img.shields.io/docker/pulls/sapcc/absent-metrics-operator)](https://hub.docker.com/r/sapcc/absent-metrics-operator)

> Project status: **alpha**. The API and user facing objects may change.

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
  description: The metric 'foo_bar' is missing. Alerts using it may not fire as intended.
```

## Installation

### Pre-compiled binaries and Docker images

See the latest [release](https://github.com/sapcc/absent-metrics-operator/releases/latest).

### Building from source

The only required build dependency is [Go](https://golang.org/).

```
$ git clone https://github.com/sapcc/absent-metrics-operator.git
$ cd absent-metrics-operator
$ make install
```

This will put the binary in `/usr/bin/`.

Alternatively, you can also build directly with the `go get` command:

```
$ go get -u github.com/sapcc/absent-metrics-operator
```

This will put the binary in `$GOPATH/bin/`.

## Usage

```
$ absent-metrics-operator --kubeconfig="$KUBECONFIG"
```

`kubeconfig` flag is only required if running outside a cluster.

For detailed usage instructions:

```
$ absent-metrics-operator --help
```

You can disable the operator for a specific `PrometheusRule` resource by adding
the following label to it:

```yaml
absent-metrics-operator/disable: true
```

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
  description: The metric '$metric' is missing. Alerts using it may not fire as intended.
```

Consider the metric `limes_successful_scrapes:rate5m` with tier `os` and
service `limes`.

Then the alert name would be `AbsentOsLimesSuccessfulScrapesRate5m`.

### Labels

**Note**: There should be at least one alert rule for a specific Prometheus
server in a namespace that has the `tier` and `service` label defined without
templating, i.e. does not use `$labels`. See caveat below.

- `tier` and `service` labels are carried over from the original alert rule
  unless the alert rule uses templating for these labels, in which case the
  default `tier` and `service` values for that Prometheus server in that
  namespace will be used.
- `severity` is always `info`.

#### Caveat

The operator determines a default `tier` and `service` for a specific
Prometheus server in a namespace by traversing through all the alert rule
definitions for that Prometheus server in that namespace. It chooses the most
common `tier` and `service` label combination that is used across these alerts
as the default values.

It is important that the operator finds a default `tier` and `service`
otherwise the operator will print an error and it will not create absent alert
rules for that specific `PrometheusRule`. It will instead requeue that resource
for later processing.
