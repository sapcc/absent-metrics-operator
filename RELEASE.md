# Release Guide

1. Ensure local `master` branch is up to date with `origin/master`.
2. Ensure all checks are passing: `make check`.
3. Update the [`CHANGELOG`](./CHANGELOG.md) as appropriate. Make sure that the format is
   consistent. We follow [semantic versioning][semver] for our releases.
4. Commit the updated changelog with message: `Release <version>`
5. Create and push a new [annotated Git tag][annotated-tag]: note that tags are prefixed
   with `v`. Use the release notes from the changelog as the tag's description.

[annotated-tag]: https://git-scm.com/book/en/v2/Git-Basics-Tagging#_annotated_tags
[semver]: https://semver.org/spec/v2.0.0.html
