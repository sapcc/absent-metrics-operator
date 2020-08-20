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

var limesLab = map[string]string{
	"tier":     "os",
	"service":  "limes",
	"severity": "info",
}

// ResMgmtOSAbsentPromRule represents the PrometheusRule that should be
// generated for the "openstack" Prometheus server in the "resmgmt" namespace.
var ResMgmtOSAbsentPromRule = monitoringv1.PrometheusRule{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "resmgmt",
		Name:      OSAbsentPromRuleName,
		Labels: map[string]string{
			"prometheus":                         "openstack",
			"type":                               "alerting-rules",
			"absent-metrics-operator/managed-by": "true",
			"tier":                               "os",
			"service":                            "limes",
		},
	},
	Spec: monitoringv1.PrometheusRuleSpec{
		Groups: []monitoringv1.RuleGroup{LimesAbsentAlertsAPIGroup, LimesAbsentAlertsRoleAssignGroup},
	},
}

// LimesAbsentAlertsAPIGroup is the rule group that holds the absent metric
// alert rules for the "openstack-limes-api.alerts" resource.
var LimesAbsentAlertsAPIGroup = monitoringv1.RuleGroup{
	Name: "openstack-limes-api.alerts/api.alerts",
	Rules: []monitoringv1.Rule{
		{
			Alert:  "AbsentOsLimesHttpRequestsTotal",
			Expr:   intstr.FromString("absent(http_requests_total)"),
			For:    "10m",
			Labels: limesLab,
			Annotations: map[string]string{
				"summary": "missing http_requests_total",
				"description": "The metric 'http_requests_total' is missing. 'OpenstackLimesHttpErrors' alert using it may not fire as intended. " +
					"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
			},
		},
		{
			Alert:  "AbsentOsLimesSuspendedScrapes",
			Expr:   intstr.FromString("absent(limes_suspended_scrapes)"),
			For:    "10m",
			Labels: limesLab,
			Annotations: map[string]string{
				"summary": "missing limes_suspended_scrapes",
				"description": "The metric 'limes_suspended_scrapes' is missing. " +
					"'OpenstackLimesSuspendedScrapes' alert using it may not fire as intended. " +
					"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
			},
		},
	},
}

// LimesAbsentAlertsRoleAssignGroup is the rule group that holds the absent
// metric alert rules for the "openstack-limes-roleassign.alerts" resource.
var LimesAbsentAlertsRoleAssignGroup = monitoringv1.RuleGroup{
	Name: "openstack-limes-roleassign.alerts/roleassignment.alerts",
	Rules: []monitoringv1.Rule{
		{
			Alert:  "AbsentOsLimesOpenstackAssignmentsPerRole",
			Expr:   intstr.FromString("absent(openstack_assignments_per_role)"),
			For:    "10m",
			Labels: limesLab,
			Annotations: map[string]string{
				"summary": "missing openstack_assignments_per_role",
				"description": "The metric 'openstack_assignments_per_role' is missing. " +
					"'OpenstackLimesUnexpectedCloudViewerRoleAssignments' alert using it may not fire as intended. " +
					"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
			},
		},
	},
}
