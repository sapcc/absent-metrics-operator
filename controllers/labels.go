package controllers

import (
	"context"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These constants are exported for reusability across packages.
const (
	LabelTier    = "tier"
	LabelService = "service"
)

const (
	annotationOperatorUpdatedAt = "absent-metrics-operator/updated-at"

	labelOperatorManagedBy = "absent-metrics-operator/managed-by"
	labelOperatorDisable   = "absent-metrics-operator/disable"

	labelNoAlertOnAbsence = "no_alert_on_absence"
	labelPrometheusServer = "prometheus"
)

// LabelOpts holds the options that define labels for an absence alert rule.
type LabelOpts struct {
	DefaultTier    string
	DefaultService string

	Keep KeepLabel
}

// KeepLabel specifies which labels to keep on an absence alert rule.
type KeepLabel map[string]bool

func keepTierServiceLabels(keep KeepLabel) bool {
	return keep[LabelTier] && keep[LabelService]
}

// labelOptsWithDefaultTierAndService finds defaults for tier and service labels for an
// AbsencePrometheusRule and returns the corresponding LabelOpts.
func (r *PrometheusRuleReconciler) labelOptsWithDefaultTierAndService(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) *LabelOpts {
	result := &LabelOpts{
		Keep: r.KeepLabel,
	}

	// Strategy 1: check if the AbsencePrometheusRule already has tier and service labels
	// defined.
	l := absencePromRule.GetLabels()
	result.DefaultTier, result.DefaultService = l[LabelTier], l[LabelService]
	if result.DefaultTier != "" && result.DefaultService != "" { // since these labels are co-dependent
		return result
	}

	// Strategy 2: try to find the tier and service label from absence alert rules.
	if len(absencePromRule.Spec.Groups) > 0 {
		result.DefaultTier, result.DefaultService = mostCommonTierAndServiceCombo(absencePromRule.Spec.Groups)
		if result.DefaultTier != "" && result.DefaultService != "" {
			return result
		}
	}

	// Strategy 3: iterate through all the alert rule definitions for the concerning
	// Prometheus server in this specific namespace.
	var listOpts client.ListOptions
	client.InNamespace(absencePromRule.GetNamespace()).ApplyToList(&listOpts)
	client.MatchingLabels{labelPrometheusServer: l[labelPrometheusServer]}.ApplyToList(&listOpts)
	var promRules monitoringv1.PrometheusRuleList
	if err := r.List(ctx, &promRules, &listOpts); err != nil {
		return nil
	}

	var rg []monitoringv1.RuleGroup
	for _, pr := range promRules.Items {
		rg = append(rg, pr.Spec.Groups...)
	}
	result.DefaultTier, result.DefaultService = mostCommonTierAndServiceCombo(rg)
	if result.DefaultTier == "" || result.DefaultService == "" {
		return nil
	}

	return result
}

func mostCommonTierAndServiceCombo(ruleGroups []monitoringv1.RuleGroup) (tier, service string) {
	// Map of tier to service to number of occurrences.
	count := make(map[string]map[string]int)
	for _, g := range ruleGroups {
		for _, r := range g.Rules {
			if r.Record != "" {
				continue // skip recording rule
			}
			t, ok := r.Labels[LabelTier]
			if !ok || strings.Contains(t, "$labels") {
				continue
			}
			s, ok := r.Labels[LabelService]
			if !ok || strings.Contains(s, "$labels") {
				continue
			}
			if count[t] == nil {
				count[t] = make(map[string]int)
			}
			count[t][s]++
		}
	}

	var i int
	for t, m := range count {
		for s, j := range m {
			if j > i {
				i = j
				tier = t
				service = s
			}
		}
	}
	return tier, service
}
