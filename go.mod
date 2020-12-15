module github.com/sapcc/absent-metrics-operator

go 1.15

// Note: ensure that the "k8s.io/*" and "github.com/prometheus/*" dependencies
// have the same versions as used by github.com/prometheus-operator/prometheus-operator.
require (
	github.com/go-kit/kit v0.10.0
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/pkg/errors v0.9.1
	github.com/prometheus-operator/prometheus-operator v0.44.1
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.44.1
	github.com/prometheus/client_golang v1.8.0
	github.com/prometheus/prometheus v1.8.2-0.20201015110737-0a7fdd3b7696
	github.com/sapcc/go-bits v0.0.0-20201203204854-32575942fc71
	golang.org/x/sync v0.0.0-20201008141435-b3e1573b7520
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.7.0
)

replace (
	// A replace directive is needed for k8s.io/client-go because Cortex (which
	// is an indirect dependency through Thanos) has a requirement on v12.0.0.
	k8s.io/client-go => k8s.io/client-go v0.19.2
	// Override the official klog package with this one. This simply replaces
	// the code in vendor/k8s.io/klog with the code of this package.
	k8s.io/klog => github.com/simonpasquier/klog-gokit v0.3.0
	k8s.io/klog/v2 => github.com/simonpasquier/klog-gokit/v2 v2.0.1
)
