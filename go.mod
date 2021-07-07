module github.com/sapcc/absent-metrics-operator

go 1.16

// Note: ensure that the "k8s.io/*" and "github.com/prometheus/*" dependencies
// have the same versions as used by github.com/prometheus-operator/prometheus-operator.
require (
	github.com/go-kit/kit v0.10.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.49.0
	github.com/prometheus-operator/prometheus-operator/pkg/client v0.49.0
	github.com/prometheus/client_golang v1.11.0
	github.com/prometheus/prometheus v1.8.2-0.20210701133801-b0944590a1c9
	github.com/sapcc/go-bits v0.0.0-20210518135053-8a9465bb1339
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	sigs.k8s.io/controller-runtime v0.9.2
)

replace (
	// Override the official klog package with this one. This simply replaces
	// the code in vendor/k8s.io/klog with the code of this package.
	k8s.io/klog => github.com/simonpasquier/klog-gokit v0.3.0
	k8s.io/klog/v2 => github.com/simonpasquier/klog-gokit/v2 v2.1.0
)
