module github.com/sapcc/absent-metrics-operator

go 1.14

// To avoid problems, make sure that the versions of k8s.io/* modules are the
// same.
require (
	github.com/coreos/prometheus-operator v0.41.0
	github.com/go-kit/kit v0.10.0
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/prometheus v1.8.2-0.20200609102542-5d7e3e970602
	github.com/sapcc/go-bits v0.0.0-20200719195243-6f202ca5296a
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v0.18.6
	sigs.k8s.io/controller-runtime v0.6.2
)
