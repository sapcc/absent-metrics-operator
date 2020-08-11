# E2E Testing

End-to-end (e2e) testing is automated testing for real user scenarios.

## Build and run tests

Prerequisites:
- `kubebuilder` control plane binaries.

The test environment expects the control plane binaries to be located in the
`bin` subdirectory. Running `make test` from the root directory of the repo
will download these binaries and run all the tests.

e2e tests are written as Go tests. All Go test techniques apply, e.g. picking
what to run, timeout length.

To run all the tests:

```
$ go test -v .
```
