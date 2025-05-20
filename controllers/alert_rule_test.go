// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var _ = Describe("Alert Rule", func() {
	logger := zap.New(zap.UseDevMode(true))
	keepLabel := KeepLabel{
		LabelSupportGroup: true,
		LabelTier:         true,
		LabelService:      true,
	}

	DescribeTable("Parsing alert rule expressions",
		func(in monitoringv1.Rule, out []monitoringv1.Rule) {
			expected := out
			actual, err := parseRule(logger, in, keepLabel)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(HaveLen(len(expected)))

			// We only check the alert name, expression, and labels. Annotations are hard-coded and
			// don't need to be checked here in unit tests; they are already checked in e2e tests.
			for i, wanted := range expected {
				got := actual[i]
				Expect(got.Alert).To(Equal(wanted.Alert))
				Expect(got.Expr).To(Equal(wanted.Expr))
				Expect(got.Labels).To(Equal(wanted.Labels))
			}
		},
		Entry("alert rule with label matching in expression and templating in labels",
			monitoringv1.Rule{
				Alert: "OpenstackKeppelPodSchedulingInsufficientMemory",
				Expr:  intstr.FromString(`sum(rate(kube_pod_failed_scheduling_memory_total{namespace="keppel"}[30m])) by (pod_name) > 0`),
				Labels: map[string]string{
					"tier":    `{{ $labels.somelabel }}`,
					"service": "keppel",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentKeppelKubePodFailedSchedulingMemoryTotal",
				Expr:  intstr.FromString(`absent(kube_pod_failed_scheduling_memory_total)`),
				Labels: map[string]string{
					"context":  "absent-metrics",
					"severity": "info",
					"service":  "keppel",
				},
			}},
		),
		Entry("alert rule with multiple usage of the same metric in expression",
			monitoringv1.Rule{
				Alert: "OpenstackKeppelPodOOMExceedingLimits",
				Expr:  intstr.FromString(`keppel_container_memory_usage_percent > 70 and predict_linear(keppel_container_memory_usage_percent[1h], 7*3600) > 100`),
				Labels: map[string]string{
					"tier":    "os",
					"service": "keppel",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentOsKeppelContainerMemoryUsagePercent",
				Expr:  intstr.FromString(`absent(keppel_container_memory_usage_percent)`),
				Labels: map[string]string{
					"context":  "absent-metrics",
					"severity": "info",
					"tier":     "os",
					"service":  "keppel",
				},
			}},
		),
		Entry("alert rule with multiple different metrics in the expression",
			monitoringv1.Rule{
				Alert: "OpenstackSwiftHealthCheck",
				Expr:  intstr.FromString(`avg(swift_recon_task_exit_code) BY (region) > 0.2 or avg(swift_dispersion_task_exit_code) BY (region) > 0.2`),
				Labels: map[string]string{
					"support_group": "not-containers",
					"tier":          "os",
					"service":       "swift",
				},
			},
			[]monitoringv1.Rule{
				{
					Alert: "AbsentNotContainersSwiftDispersionTaskExitCode",
					Expr:  intstr.FromString(`absent(swift_dispersion_task_exit_code)`),
					Labels: map[string]string{
						"context":       "absent-metrics",
						"severity":      "info",
						"support_group": "not-containers",
						"tier":          "os",
						"service":       "swift",
					},
				},
				{
					Alert: "AbsentNotContainersSwiftReconTaskExitCode",
					Expr:  intstr.FromString(`absent(swift_recon_task_exit_code)`),
					Labels: map[string]string{
						"context":       "absent-metrics",
						"severity":      "info",
						"support_group": "not-containers",
						"tier":          "os",
						"service":       "swift",
					},
				},
			},
		),
		Entry("complex expression test case 1",
			monitoringv1.Rule{
				Alert: "OpenstackSwiftUsedSpace",
				Expr:  intstr.FromString(`max(predict_linear(global:swift_cluster_storage_used_percent_average[1w], 60 * 60 * 24 * 30)) > 0.8`),
				Labels: map[string]string{
					"support_group": "not-containers",
					"tier":          "os",
					"service":       "swift",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentNotContainersSwiftGlobalSwiftClusterStorageUsedPercentAverage",
				Expr:  intstr.FromString(`absent(global:swift_cluster_storage_used_percent_average)`),
				Labels: map[string]string{
					"context":       "absent-metrics",
					"severity":      "info",
					"support_group": "not-containers",
					"tier":          "os",
					"service":       "swift",
				},
			}},
		),
		Entry("complex expression test case 2",
			monitoringv1.Rule{
				Alert: "OpenstackLimesHttpErrors",
				Expr:  intstr.FromString(`sum(increase(http_requests_total{kubernetes_namespace="limes",code=~"5.*"}[1h])) by (kubernetes_name) > 0`),
				Labels: map[string]string{
					"support_group": "containers",
					"service":       "limes",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentContainersLimesHttpRequestsTotal",
				Expr:  intstr.FromString(`absent(http_requests_total)`),
				Labels: map[string]string{
					"context":       "absent-metrics",
					"severity":      "info",
					"support_group": "containers",
					"service":       "limes",
				},
			}},
		),
		Entry("alert rule that uses label matching against the internal '__name__' label in expression",
			monitoringv1.Rule{
				Alert: "OpenstackLimesSuspendedScrapes",
				Expr:  intstr.FromString(`sum(increase({__name__=~'limes_suspended_scrapes'}[15m])) BY (os_cluster, service, service_name) > 0`),
				Labels: map[string]string{
					"support_group": "containers",
					"service":       "limes",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentContainersLimesSuspendedScrapes",
				Expr:  intstr.FromString(`absent(limes_suspended_scrapes)`),
				Labels: map[string]string{
					"context":       "absent-metrics",
					"severity":      "info",
					"support_group": "containers",
					"service":       "limes",
				},
			}},
		),
		Entry("alert rule that already uses 'absent' function for the metric used in the expression",
			monitoringv1.Rule{
				Alert: "OpenstackLimesFailedScrapes",
				Expr:  intstr.FromString(`absent(limes_failed_scrapes) or sum(increase(limes_failed_scrapes[5m])) BY (os_cluster, service, service_name) > 0`),
				Labels: map[string]string{
					"support_group": "containers",
					"service":       `{{ $labels.service_name }}`,
				},
			},
			nil, // no absence alert rules should be generated for this alert
		),
		Entry("alert rule that uses 'absent' function but for a different metric in the expression",
			monitoringv1.Rule{
				Alert: "OpenstackLimesUnexpectedServiceRoleAssignments",
				Expr:  intstr.FromString(`absent(openstack_assignments_per_service{service_name="service"}) or max(openstack_assignments_per_role{role_name="resource_service"}) > 1`),
				Labels: map[string]string{
					"support_group": "containers",
					"service":       "limes",
				},
			},
			[]monitoringv1.Rule{{
				Alert: "AbsentContainersLimesOpenstackAssignmentsPerRole",
				Expr:  intstr.FromString(`absent(openstack_assignments_per_role)`),
				Labels: map[string]string{
					"context":       "absent-metrics",
					"severity":      "info",
					"support_group": "containers",
					"service":       "limes",
				},
			}},
		),
		Entry("alert rule with 'no_alert_on_absence' label",
			monitoringv1.Rule{
				Alert: "OpenstackSwiftMismatchedRings",
				Expr:  intstr.FromString(`(swift_cluster_md5_not_matched{kind="ring"} - swift_cluster_md5_errors{kind="ring"}) > 0`),
				Labels: map[string]string{
					"support_group":       "not-containers",
					"tier":                "os",
					"service":             "swift",
					"no_alert_on_absence": "true",
				},
			},
			nil, // absence alerts are not generated for record rules
		),
		Entry("record rule",
			monitoringv1.Rule{
				Record: "predict_linear_global_cluster_storage_used_percent_average",
				Expr:   intstr.FromString(`max(predict_linear(global:swift_cluster_storage_used_percent_average[1w], 60 * 60 * 24 * 30)) > 0.8`),
				Labels: map[string]string{
					"support_group": "not-containers",
					"tier":          "os",
					"service":       "swift",
				},
			},
			nil, // absence alerts are not generated for record rules
		),
	)
})
