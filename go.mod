module github.com/sapcc/absent-metrics-operator

go 1.14

// To avoid problems, make sure that the versions of k8s and Prometheus modules
// are the same as used by the prometheus-operator.
require (
	github.com/coreos/prometheus-operator v0.41.0
	github.com/go-kit/kit v0.10.0
	github.com/prometheus/prometheus v1.8.2-0.20200609102542-5d7e3e970602
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v0.18.3
	k8s.io/klog/v2 v2.3.0
)
