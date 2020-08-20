# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2020-08-20

## Removed

- `playbook` label.

## [0.3.0] - 2020-08-20

### Added

- Parse vector selectors that use label matching against the internal
  `__name__` label.

## [0.2.0] - 2020-08-18

### Added

- Operator can be disabled for a specific alert rule.
- `playbook` label to absent metric alerts.
- `keep-labels` flag for specifying which labels to carry over from alert
  rules.

## [0.1.0] - 2020-08-13

### Added

- Initial release.

[unreleased]: https://github.com/sapcc/absent-metrics-operator/compare/v0.4.0...HEAD
[0.4.0]: https://github.com/sapcc/absent-metrics-operator/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/sapcc/absent-metrics-operator/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/sapcc/absent-metrics-operator/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/sapcc/absent-metrics-operator/releases/tag/v0.1.0
