// Copyright 2020 SAP SE
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/sapcc/absent-metrics-operator/controllers"
	//+kubebuilder:scaffold:imports
)

// This info identifies a specific build of the operator. It is set at compile time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "now"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	//+kubebuilder:scaffold:scheme
}

func main() {
	var (
		debug                bool
		metricsAddr          string
		probeAddr            string
		enableLeaderElection bool
		keepLabel            labelsMap
	)
	flag.BoolVar(&debug, "debug", false, "Alias for '-zap-devel' flag.")
	// Port `9659` has been allocated for absent metrics operator: https://github.com/prometheus/prometheus/wiki/Default-port-allocations
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":9659", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.Var(keepLabel, "keep-labels", "A comma-separated list of labels to retain from the original alert rule. "+
		fmt.Sprintf("(default %q)", labelsMap{controllers.LabelTier: true, controllers.LabelService: true}))
	opts := zap.Options{
		Development: debug,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "62fc53df.cloud.sap",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	controllers.RegisterMetrics()

	if err = (&controllers.PrometheusRuleReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		KeepLabel: controllers.KeepLabel(keepLabel),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "PrometheusRule")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager", "version", version, "git-commit", commit, "build-date", date)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// labelsMap type is a wrapper around controllers.KeepLabel. It is used for the
// `keep-labels` flag to convert a comma-separated string into a map.
type labelsMap controllers.KeepLabel

// String implements the flag.Value interface.
func (lm labelsMap) String() string {
	list := make([]string, 0, len(lm))
	for k := range lm {
		list = append(list, k)
	}
	return strings.Join(list, ",")
}

// Set implements the flag.Value interface.
func (lm labelsMap) Set(in string) error {
	lm = make(labelsMap)
	list := strings.Split(in, ",")
	for _, v := range list {
		lm[strings.TrimSpace(v)] = true
	}

	// Validate
	if lm[controllers.LabelTier] || lm[controllers.LabelService] {
		if !lm[controllers.LabelTier] && !lm[controllers.LabelService] {
			return errors.New("labels 'tier' and 'service' are co-dependent: use both or neither")
		}
	}

	return nil
}
