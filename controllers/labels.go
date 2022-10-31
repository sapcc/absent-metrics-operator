// Copyright 2022 SAP SE
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
	"context"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These constants are exported for reusability across packages.
const (
	LabelCCloudSupportGroup = "ccloud/support-group"
	LabelCCloudService      = "ccloud/service"

	LabelSupportGroup = "support_group"
	LabelTier         = "tier"
	LabelService      = "service"
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
	DefaultSupportGroup string
	DefaultTier         string
	DefaultService      string

	Keep KeepLabel
}

// KeepLabel specifies which labels to keep on an absence alert rule.
type KeepLabel map[string]bool

func keepCCloudLabels(keep KeepLabel) bool {
	return keep[LabelSupportGroup] && keep[LabelTier] && keep[LabelService]
}

// labelOptsWithCCloudDefaults finds defaults for support group and service labels for an
// AbsencePrometheusRule and returns the corresponding LabelOpts.
//
//nolint:dupl
func (r *PrometheusRuleReconciler) labelOptsWithCCloudDefaults(
	ctx context.Context,
	absencePromRule *monitoringv1.PrometheusRule,
) *LabelOpts {

	result := &LabelOpts{
		Keep: r.KeepLabel,
	}

	// Strategy 1: check if the AbsencePrometheusRule already has support group and
	// service labels defined.
	l := absencePromRule.GetLabels()
	result.DefaultSupportGroup, result.DefaultService = l[LabelCCloudSupportGroup], l[LabelCCloudService]
	if result.DefaultSupportGroup != "" && result.DefaultService != "" {
		return result
	}

	// Strategy 2: try to find the support group and service label from absence alert rules.
	if len(absencePromRule.Spec.Groups) > 0 {
		result.DefaultSupportGroup, result.DefaultService = mostCommonSupportGroupAndServiceCombo(absencePromRule.Spec.Groups)
		if result.DefaultSupportGroup != "" && result.DefaultService != "" {
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
	result.DefaultSupportGroup, result.DefaultService = mostCommonSupportGroupAndServiceCombo(rg)
	if result.DefaultSupportGroup == "" || result.DefaultService == "" {
		return nil
	}

	return result
}

// labelOptsWithDefaultTierAndService finds defaults for tier and service labels for an
// AbsencePrometheusRule and returns the corresponding LabelOpts.
//
//nolint:dupl
func (r *PrometheusRuleReconciler) labelOptsWithDefaultTierAndService(
	ctx context.Context,
	absencePromRule *monitoringv1.PrometheusRule,
) *LabelOpts {

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

//nolint:dupl
func mostCommonSupportGroupAndServiceCombo(ruleGroups []monitoringv1.RuleGroup) (supportGroup, service string) {
	// Map of support group to service to number of occurrences.
	count := make(map[string]map[string]int)
	for _, g := range ruleGroups {
		for _, r := range g.Rules {
			if r.Record != "" {
				continue // skip recording rule
			}
			t, ok := r.Labels[LabelSupportGroup]
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
	for sg, m := range count {
		for s, j := range m {
			if j > i {
				i = j
				supportGroup = sg
				service = s
			}
		}
	}
	return supportGroup, service
}

//nolint:dupl
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
