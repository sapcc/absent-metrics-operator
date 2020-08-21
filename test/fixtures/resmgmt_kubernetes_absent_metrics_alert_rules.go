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

package fixtures

import (
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var kepLab = map[string]string{
	"tier":     "os",
	"service":  "keppel",
	"severity": "info",
	"context":  "absent-metrics",
}

// ResMgmtK8sAbsentPromRule represents the PrometheusRule that should be
// generated for the "kubernetes" Prometheus server in the "resmgmt" namespace.
var ResMgmtK8sAbsentPromRule = monitoringv1.PrometheusRule{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "resmgmt",
		Name:      K8sAbsentPromRuleName,
		Labels: map[string]string{
			"prometheus":                         "kubernetes",
			"type":                               "alerting-rules",
			"absent-metrics-operator/managed-by": "true",
			"tier":                               "os",
			"service":                            "keppel",
		},
	},
	Spec: monitoringv1.PrometheusRuleSpec{
		Groups: []monitoringv1.RuleGroup{
			{
				Name: "kubernetes-keppel.alerts/keppel.alerts",
				Rules: []monitoringv1.Rule{
					{
						Alert:  "AbsentOsKeppelKubePodFailedSchedulingMemoryTotal",
						Expr:   intstr.FromString("absent(kube_pod_failed_scheduling_memory_total)"),
						For:    "10m",
						Labels: kepLab,
						Annotations: map[string]string{
							"summary": "missing kube_pod_failed_scheduling_memory_total",
							"description": "The metric 'kube_pod_failed_scheduling_memory_total' is missing. " +
								"'OpenstackKeppelPodSchedulingInsufficientMemory' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
					{
						Alert:  "AbsentOsKeppelContainerMemoryUsagePercent",
						Expr:   intstr.FromString("absent(keppel_container_memory_usage_percent)"),
						For:    "10m",
						Labels: kepLab,
						Annotations: map[string]string{
							"summary": "missing keppel_container_memory_usage_percent",
							"description": "The metric 'keppel_container_memory_usage_percent' is missing. " +
								"'OpenstackKeppelPodOOMExceedingLimits' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
				},
			},
		},
	},
}
