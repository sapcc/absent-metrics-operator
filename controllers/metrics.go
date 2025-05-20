// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/types"
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

func setReconcileGauge(key types.NamespacedName) {
	gauge := successfulReconcileTime.WithLabelValues(key.Namespace, key.Name)
	if IsTest {
		gauge.Set(1)
	} else {
		gauge.SetToCurrentTime()
	}
}

func deleteReconcileGauge(key types.NamespacedName) {
	successfulReconcileTime.DeleteLabelValues(key.Namespace, key.Name)
}
