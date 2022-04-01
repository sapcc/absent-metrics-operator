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

package controllers

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// IsTest is set by the test suite during testing.
var IsTest bool

// RegisterMetrics registers all the metrics.
// If IsTest is true then it will also return a *prometheus.Registry than can be used in
// the test suite otherwise nil is returned.
func RegisterMetrics() *prometheus.Registry {
	if IsTest {
		// We don't use `controllers.RegisterMetrics()` here as that will also include
		// metrics related to the controller which will make testing with fixtures
		// difficult.
		reg := prometheus.NewPedanticRegistry()
		reg.MustRegister(successfulReconcileTime)
		return reg
	}
	metrics.Registry.MustRegister(successfulReconcileTime)
	return nil
}

var successfulReconcileTime = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "absent_metrics_operator_successful_reconcile_time",
		Help: "The time at which a specific PrometheusRule was successfully reconciled by the operator.",
	},
	[]string{"prometheusrule_namespace", "prometheusrule_name"},
)

func setReconcileGauge(namespace, name string, now time.Time) {
	gauge := successfulReconcileTime.WithLabelValues(namespace, name)
	if IsTest {
		gauge.Set(1)
	} else {
		// The following is the same as doing gauge.SetToCurrentTime() but we do it
		// manually so that we can use our own time.Time.
		gauge.Set(float64(now.UnixNano() / 1e9))
	}
}

func deleteReconcileGauge(namespace, name string) {
	successfulReconcileTime.DeleteLabelValues(namespace, name)
}
