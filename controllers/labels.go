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

// defaultSupportGroupAndServiceLabels finds defaults for support group and service labels for an
// AbsencePrometheusRule and returns the corresponding LabelOpts.
func (r *PrometheusRuleReconciler) labelOptsWithCCloudDefaults(
	ctx context.Context,
	promRule *monitoringv1.PrometheusRule,
) (LabelOpts, error) {

	opts := LabelOpts{Keep: r.KeepLabel}

	newIfCurrentEmpty := func(currentVal, newVal string) string {
		if currentVal != "" {
			return currentVal
		}
		return newVal
	}
	foundLabels := func() bool {
		return opts.DefaultSupportGroup != "" && opts.DefaultService != "" && opts.DefaultTier != ""
	}

	// Strategy 1: check if the PrometheusRule already has the required labels.
	l := promRule.GetLabels()
	opts.DefaultSupportGroup = l[LabelCCloudSupportGroup]
	opts.DefaultService = l[LabelCCloudService]
	// Try old CCloud service label naming.
	opts.DefaultService = newIfCurrentEmpty(opts.DefaultService, l[LabelService])
	opts.DefaultTier = l[LabelTier]
	if foundLabels() {
		return opts, nil
	}

	// Strategy 2: iterate through all the alert rule definitions.
	if len(promRule.Spec.Groups) > 0 {
		sg, s := mostCommonSupportGroupAndServiceCombo(promRule.Spec.Groups)
		opts.DefaultSupportGroup = newIfCurrentEmpty(opts.DefaultSupportGroup, sg)
		opts.DefaultService = newIfCurrentEmpty(opts.DefaultService, s)

		t, s := mostCommonTierAndServiceCombo(promRule.Spec.Groups)
		opts.DefaultTier = newIfCurrentEmpty(opts.DefaultTier, t)
		opts.DefaultService = newIfCurrentEmpty(opts.DefaultService, s)

		if foundLabels() {
			return opts, nil
		}
	}

	// Strategy 3: iterate through all the alert rule definitions for the concerning
	// Prometheus server in this specific namespace.
	var listOpts client.ListOptions
	client.InNamespace(promRule.GetNamespace()).ApplyToList(&listOpts)
	client.MatchingLabels{labelPrometheusServer: l[labelPrometheusServer]}.ApplyToList(&listOpts)
	var promRules monitoringv1.PrometheusRuleList
	if err := r.List(ctx, &promRules, &listOpts); err != nil {
		return opts, err
	}
	var rg []monitoringv1.RuleGroup
	for _, pr := range promRules.Items {
		if _, ok := pr.Labels[labelOperatorManagedBy]; ok {
			continue // skip absence alert rules
		}
		rg = append(rg, pr.Spec.Groups...)
	}

	sg, s := mostCommonSupportGroupAndServiceCombo(rg)
	opts.DefaultSupportGroup = newIfCurrentEmpty(opts.DefaultSupportGroup, sg)
	opts.DefaultService = newIfCurrentEmpty(opts.DefaultService, s)

	t, s := mostCommonTierAndServiceCombo(rg)
	opts.DefaultTier = newIfCurrentEmpty(opts.DefaultTier, t)
	opts.DefaultService = newIfCurrentEmpty(opts.DefaultService, s)

	return opts, nil
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
			sg, ok := r.Labels[LabelSupportGroup]
			if !ok || strings.Contains(sg, "$labels") {
				continue
			}
			s, ok := r.Labels[LabelService]
			if !ok || strings.Contains(s, "$labels") {
				continue
			}
			if count[sg] == nil {
				count[sg] = make(map[string]int)
			}
			count[sg][s]++
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

func getCCloudLabels(pr *monitoringv1.PrometheusRule) map[string]string {
	l := make(map[string]string)
	for k, v := range pr.GetLabels() {
		if k == LabelCCloudSupportGroup ||
			k == LabelCCloudService ||
			k == LabelService ||
			k == LabelTier {
			l[k] = v
		}
	}
	return l
}

// updateLabel is used to update a specific label in the label map.
// If a new value is provided then the label at the specific key will be updated with the new value.
// If value is an empty string then the label at the specific key will be deleted from the map.
func updateLabel(m map[string]string, key, val string) {
	if val == "" {
		delete(m, key)
		return
	}
	m[key] = val
}
