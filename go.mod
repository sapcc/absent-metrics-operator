module github.com/sapcc/absent-metrics-operator

go 1.14

require (
	github.com/coreos/prometheus-operator v0.40.0
	github.com/go-kit/kit v0.10.0
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	k8s.io/apimachinery v0.18.6
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/klog/v2 v2.3.0
)

// To avoid problems, make sure that the k8s components use the same version as
// the prometheus-operator.
replace (
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.2
	k8s.io/client-go => k8s.io/client-go v0.18.2
)
