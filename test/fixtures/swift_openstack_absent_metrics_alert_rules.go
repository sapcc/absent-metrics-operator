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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var swiftLab = map[string]string{
	"tier":     "os",
	"service":  "swift",
	"severity": "info",
	"context":  "absent-metrics",
}

// SwiftOSAbsentPromRule represents the PrometheusRule that should be generated
// for the "openstack" Prometheus server in the "swift" namespace.
var SwiftOSAbsentPromRule = monitoringv1.PrometheusRule{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "swift",
		Name:      OSAbsentPromRuleName,
		Labels: map[string]string{
			"prometheus":                         "openstack",
			"type":                               "alerting-rules",
			"absent-metrics-operator/managed-by": "true",
			"tier":                               "os",
			"service":                            "swift",
		},
	},
	Spec: monitoringv1.PrometheusRuleSpec{
		Groups: []monitoringv1.RuleGroup{
			{
				Name: "openstack-swift.alerts/swift.alerts",
				Rules: []monitoringv1.Rule{
					{
						Alert:  "AbsentOsSwiftGlobalSwiftClusterStorageUsedPercentAverage",
						Expr:   intstr.FromString("absent(global:swift_cluster_storage_used_percent_average)"),
						For:    "10m",
						Labels: swiftLab,
						Annotations: map[string]string{
							"summary": "missing global:swift_cluster_storage_used_percent_average",
							"description": "The metric 'global:swift_cluster_storage_used_percent_average' is missing. " +
								"'OpenstackSwiftUsedSpace' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
					{
						Alert:  "AbsentOsSwiftDispersionTaskExitCode",
						Expr:   intstr.FromString("absent(swift_dispersion_task_exit_code)"),
						For:    "10m",
						Labels: swiftLab,
						Annotations: map[string]string{
							"summary": "missing swift_dispersion_task_exit_code",
							"description": "The metric 'swift_dispersion_task_exit_code' is missing. " +
								"'OpenstackSwiftHealthCheck' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
					{
						Alert:  "AbsentOsSwiftReconTaskExitCode",
						Expr:   intstr.FromString("absent(swift_recon_task_exit_code)"),
						For:    "10m",
						Labels: swiftLab,
						Annotations: map[string]string{
							"summary": "missing swift_recon_task_exit_code",
							"description": "The metric 'swift_recon_task_exit_code' is missing. " +
								"'OpenstackSwiftHealthCheck' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
					{
						Alert:  "AbsentOsSwiftClusterMd5NotMatched",
						Expr:   intstr.FromString("absent(swift_cluster_md5_not_matched)"),
						For:    "10m",
						Labels: swiftLab,
						Annotations: map[string]string{
							"summary": "missing swift_cluster_md5_not_matched",
							"description": "The metric 'swift_cluster_md5_not_matched' is missing. " +
								"'OpenstackSwiftMismatchedConfig' alert using it may not fire as intended. " +
								"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
						},
					},
				},
			},
		},
	},
}
