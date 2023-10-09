# Release Guide

1. Ensure local `master` branch is up to date with `origin/master`.
2. Create the Evntest environment with `make prepare-envtest-binaries`. Ensure all checks are passing: `make check`.
3. Update the [`changelog`](./CHANGELOG.md). Make sure that the format is consistent
   especially the version heading. We follow [semantic versioning][semver] for our
   releases.
4. Commit the updated changelog with message: `Release <version>` .
5. Create and push a new Git tag: note that we prefix our Git tags with `v` .
6. [Draft](https://github.com/sapcc/absent-metrics-operator/releases/new) a new release
   for the new tag. Use the release notes from the changelog as the release's description.

[semver]: https://semver.org/spec/v2.0.0.html
