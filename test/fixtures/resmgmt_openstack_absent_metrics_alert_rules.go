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
				"summary":     "missing http_requests_total",
				"description": "The metric 'http_requests_total' is missing",
			},
		},
		{
			Alert:  "AbsentOsLimesSuspendedScrapes",
			Expr:   intstr.FromString("absent(limes_suspended_scrapes)"),
			For:    "10m",
			Labels: limesLab,
			Annotations: map[string]string{
				"summary":     "missing limes_suspended_scrapes",
				"description": "The metric 'limes_suspended_scrapes' is missing",
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
				"summary":     "missing openstack_assignments_per_role",
				"description": "The metric 'openstack_assignments_per_role' is missing",
			},
		},
	},
}
