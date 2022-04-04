# Changelog

All notable changes to the absent metrics operator will be documented in this file.

The sections should follow the order `Added`, `Changed`, `Fixed`, `Removed`, and `Deprecated`.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/sapcc/absent-metrics-operator/compare/v0.7.3...HEAD)

### Added

- Use Kubebuilder for scaffolding.
- `absent-metrics-operator/updated-at` annotation to operator generated `PrometheusRule`
  resources which specifies the time (UTC) at which this resource was updated by the
  operator.

### Changed

- Update Ginkgo testing framework to v2.
- Updated all dependencies to their latest version.

## 0.7.3 - 2021-11-29

### Changed

- Updated all dependencies to their latest version.

## 0.7.2 - 2021-09-29

### Changed

- Updated Go to `1.17` and all dependencies to their latest version.

## 0.7.1 - 2020-11-17

### Fixed

- Clean up of PrometheusRule resources for which the operator is disabled.

## 0.7.0 - 2020-09-28

### Fixed

- Clean up orphaned gauge metrics.

## 0.6.0 - 2020-08-26

### Added

- Manual maintenance task.

### Fixed

- Delete timeseries concerning `PrometheusRules` that no longer exist.

## 0.5.2 - 2020-08-21

### Fixed

- A bug that was introduced in the previous release.

## 0.5.1 - 2020-08-21

### Changed

- Prevent superfluous processing if the resource doesn't have any alert rules.

## 0.5.0 - 2020-08-21

### Added

- `context` label to absence alerts.

## 0.4.0 - 2020-08-20

### Removed

- `playbook` label from absence alerts.

## 0.3.0 - 2020-08-20

### Added

- Parse vector selectors that use label matching against the internal
  `__name__` label.

## 0.2.0 - 2020-08-18

### Added

- Operator can be disabled for a specific alert rule.
- `playbook` label to absence alerts.
- `keep-labels` flag for specifying which labels to carry over from alert
  rules.

## 0.1.0 - 2020-08-13

### Added

- Initial release.
