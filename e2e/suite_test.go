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

package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/yaml"

	"github.com/sapcc/absent-metrics-operator/controllers"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	logger logr.Logger

	k8sClient client.Client
	testEnv   *envtest.Environment
	reg       *prometheus.Registry

	ctx    context.Context
	wg     *errgroup.Group
	cancel context.CancelFunc

	keepLabel = controllers.KeepLabel{
		controllers.LabelSupportGroup: true,
		controllers.LabelTier:         true,
		controllers.LabelService:      true,
	}
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "e2e Test Suite")
}

var _ = BeforeSuite(func() {
	controllers.IsTest = true

	logger = zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true))
	logf.SetLogger(logger)

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{"crd"},
		ErrorIfCRDPathMissing: true,
	}
	cfg := checkErrAndReturnResult(testEnv.Start())

	Expect(monitoringv1.AddToScheme(scheme.Scheme)).To(Succeed())

	mgr := checkErrAndReturnResult(ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
	}))

	reg = controllers.RegisterMetrics()

	Expect((&controllers.PrometheusRuleReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Log:                ctrl.Log.WithName("controller").WithName("prometheusrule"),
		KeepLabel:          keepLabel,
		PrometheusRuleName: checkErrAndReturnResult(controllers.CreateAbsencePromRuleNameGenerator(controllers.DefaultAbsencePromRuleNameTemplate)),
	}).SetupWithManager(mgr)).To(Succeed())

	//+kubebuilder:scaffold:scheme

	k8sClient = checkErrAndReturnResult(client.New(cfg, client.Options{Scheme: scheme.Scheme}))

	// We start the controller before adding objects since the items are
	// queued by the controller sequentially and we depend on this behavior in
	// our mock assertions.
	By("starting manager")
	ctx, cancel = context.WithCancel(ctrl.SetupSignalHandler())
	wg, ctx = errgroup.WithContext(ctx)
	wg.Go(func() error {
		return mgr.Start(ctx)
	})

	By("adding mock PrometheusRule resources")
	Expect(addMockPrometheusRules(ctx)).To(Succeed())

	// High duration for sleep is needed otherwise test runs in CI fail.
	time.Sleep(1 * time.Second)
})

var _ = AfterSuite(func() {
	By("stopping manager")
	cancel()
	Expect(wg.Wait()).To(Succeed())

	By("tearing down the test environment")
	Expect(testEnv.Stop()).To(Succeed())
})

///////////////////////////////////////////////////////////////////////////////
// Helper functions

func addMockPrometheusRules(ctx context.Context) error {
	mockDir := filepath.Join("fixtures", "start-data")
	mockFiles, err := os.ReadDir(mockDir)
	if err != nil {
		return err
	}

	for _, file := range mockFiles {
		var pr monitoringv1.PrometheusRule
		b, err := os.ReadFile(filepath.Join(mockDir, file.Name()))
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(b, &pr)
		if err != nil {
			return err
		}

		// Create namespace if it doesn't exist already.
		var ns corev1.Namespace
		err = k8sClient.Get(ctx, client.ObjectKey{Name: pr.Namespace}, &ns)
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			ns.Name = pr.Namespace
			err = k8sClient.Create(ctx, &ns)
		}
		if err != nil {
			return err
		}

		// Create PrometheusRule resource.
		err = k8sClient.Create(ctx, &pr)
		if err != nil {
			return err
		}
	}

	return nil
}
