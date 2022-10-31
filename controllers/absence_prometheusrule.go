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
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const absencePromRuleNameSuffix = "-absent-metric-alert-rules"

// AbsencePrometheusRuleName returns the name of an AbsencePrometheusRule resource that
// holds the absence alert rules concerning a specific Prometheus server (e.g. openstack, kubernetes, etc.).
func AbsencePrometheusRuleName(promServer string) string {
	return fmt.Sprintf("%s%s", promServer, absencePromRuleNameSuffix)
}

func (r *PrometheusRuleReconciler) newAbsencePrometheusRule(namespace, promServer string) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AbsencePrometheusRuleName(promServer),
			Namespace: namespace,
			Labels: map[string]string{
				// Add a label that identifies that this PrometheusRule resource is
				// created and managed by this operator.
				labelOperatorManagedBy: "true",
				labelPrometheusServer:  promServer,
				"type":                 "alerting-rules",
			},
		},
	}
}

func (r *PrometheusRuleReconciler) getExistingAbsencePrometheusRule(
	ctx context.Context,
	namespace, promServer string,
) (*monitoringv1.PrometheusRule, error) {

	var absencePromRule monitoringv1.PrometheusRule
	nsName := types.NamespacedName{Namespace: namespace, Name: AbsencePrometheusRuleName(promServer)}
	if err := r.Get(ctx, nsName, &absencePromRule); err != nil {
		return nil, err
	}
	return &absencePromRule, nil
}

func sortRuleGroups(absencePromRule *monitoringv1.PrometheusRule) {
	// Sort rule groups for consistent test results.
	sort.SliceStable(absencePromRule.Spec.Groups, func(i, j int) bool {
		return absencePromRule.Spec.Groups[i].Name < absencePromRule.Spec.Groups[j].Name
	})
}

func updateAnnotationTime(absencePromRule *monitoringv1.PrometheusRule) {
	now := time.Now()
	if IsTest {
		now = time.Unix(1, 0)
	}
	if absencePromRule.Annotations == nil {
		absencePromRule.Annotations = make(map[string]string)
	}
	absencePromRule.Annotations[annotationOperatorUpdatedAt] = now.UTC().Format(time.RFC3339)
}

func (r *PrometheusRuleReconciler) createAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	sortRuleGroups(absencePromRule)
	updateAnnotationTime(absencePromRule)
	if err := r.Create(ctx, absencePromRule, &client.CreateOptions{}); err != nil {
		return err
	}

	r.Log.Info("successfully created AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

func (r *PrometheusRuleReconciler) updateAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	sortRuleGroups(absencePromRule)
	updateAnnotationTime(absencePromRule)
	if err := r.Update(ctx, absencePromRule, &client.UpdateOptions{}); err != nil {
		return err
	}

	r.Log.Info("successfully updated AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

func (r *PrometheusRuleReconciler) deleteAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	if err := r.Delete(ctx, absencePromRule, &client.DeleteOptions{}); err != nil {
		return err
	}

	r.Log.Info("successfully deleted AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

// cleanUpOrphanedAbsenceAlertRules deletes the absence alert rules for a PrometheusRule
// resource.
//
// We use this when a PrometheusRule resource has been deleted or if the
// 'absent-metrics-operator/disable' is set to 'true'.
func (r *PrometheusRuleReconciler) cleanUpOrphanedAbsenceAlertRules(
	ctx context.Context,
	promRule types.NamespacedName,
	promServer string,
) error {
	// Step 1: find the corresponding AbsencePrometheusRule that needs to be cleaned up.
	var aPRToClean *monitoringv1.PrometheusRule
	if promServer != "" {
		var err error
		if aPRToClean, err = r.getExistingAbsencePrometheusRule(ctx, promRule.Namespace, promServer); err != nil {
			return err
		}
	} else {
		// Since we don't know the Prometheus server for this PrometheusRule therefore we
		// have to list all AbsencePrometheusRules in its namespace and find the specific
		// AbsencePrometheusRule that contains the absence alert rules that were generated
		// for this PrometheusRule.
		var listOpts client.ListOptions
		client.InNamespace(promRule.Namespace).ApplyToList(&listOpts)
		client.HasLabels{labelOperatorManagedBy}.ApplyToList(&listOpts)
		var absencePromRules monitoringv1.PrometheusRuleList
		if err := r.List(ctx, &absencePromRules, &listOpts); err != nil {
			return err
		}

		for _, aPR := range absencePromRules.Items {
			for _, g := range aPR.Spec.Groups {
				n := promRulefromAbsenceRuleGroupName(g.Name)
				if n != "" && n == promRule.Name {
					aPRToClean = aPR
					break
				}
			}
		}
	}
	if aPRToClean == nil {
		return errors.New("could not find the corresponding AbsencePrometheusRule")
	}

	// Step 2: iterate through the AbsenceRuleGroups, skip those that were generated for
	// this PrometheusRule and keep the rest as is.
	oldRuleGroups := aPRToClean.Spec.Groups
	newRuleGroups := make([]monitoringv1.RuleGroup, 0, len(oldRuleGroups))
	for _, g := range oldRuleGroups {
		n := promRulefromAbsenceRuleGroupName(g.Name)
		if n != "" && n == promRule.Name {
			continue
		}
		newRuleGroups = append(newRuleGroups, g)
	}
	if reflect.DeepEqual(oldRuleGroups, newRuleGroups) {
		return nil
	}

	// Step 3: if, after the cleanup, the AbsencePrometheusRule ends up being empty then
	// delete it otherwise update.
	aPRToClean.Spec.Groups = newRuleGroups
	if len(aPRToClean.Spec.Groups) == 0 {
		return r.deleteAbsencePrometheusRule(ctx, aPRToClean)
	}
	return r.updateAbsencePrometheusRule(ctx, aPRToClean)
}

// cleanUpAbsencePrometheusRule checks an AbsencePrometheusRule to see if it contains
// absence alert rules for a PrometheusRule that no longer exists or for a resource that
// has the 'absent-metrics-operator/disable' label. If such rules are found then they are
// deleted.
func (r *PrometheusRuleReconciler) cleanUpAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	// Step 1: get names of all PrometheusRule resources.
	namespace := absencePromRule.GetNamespace()
	var listOpts client.ListOptions
	client.InNamespace(namespace).ApplyToList(&listOpts)
	var promRules monitoringv1.PrometheusRuleList
	if err := r.List(ctx, &promRules, &listOpts); err != nil {
		return err
	}
	prNames := make(map[string]bool)
	for _, pr := range promRules.Items {
		prNames[pr.GetName()] = true
	}

	// Step 2: iterate through all the AbsencePrometheusRule's RuleGroups and remove those
	// that don't belong to any PrometheusRule.
	newRuleGroups := make([]monitoringv1.RuleGroup, 0, len(absencePromRule.Spec.Groups))
	for _, g := range absencePromRule.Spec.Groups {
		n := promRulefromAbsenceRuleGroupName(g.Name)
		if !prNames[n] {
			continue
		}
		newRuleGroups = append(newRuleGroups, g)
	}
	if reflect.DeepEqual(absencePromRule.Spec.Groups, newRuleGroups) {
		return nil
	}

	// Step 3: if, after the cleanup, the AbsencePrometheusRule ends up being empty then
	// delete it otherwise update.
	absencePromRule.Spec.Groups = newRuleGroups
	if len(absencePromRule.Spec.Groups) == 0 {
		return r.deleteAbsencePrometheusRule(ctx, absencePromRule)
	}
	return r.updateAbsencePrometheusRule(ctx, absencePromRule)
}

// updateAbsenceAlertRules generates absence alert rules for the given PrometheusRule and
// adds them to the corresponding AbsencePrometheusRule.
func (r *PrometheusRuleReconciler) updateAbsenceAlertRules(ctx context.Context, promRule *monitoringv1.PrometheusRule) error {
	promRuleName := promRule.GetName()
	namespace := promRule.GetNamespace()
	log := r.Log.WithValues("name", promRuleName, "namespace", namespace)

	// Step 1: find the Prometheus server for this resource.
	promRuleLabels := promRule.GetLabels()
	promServer, ok := promRuleLabels["prometheus"]
	if !ok {
		// Normally this shouldn't happen but just in case that it does.
		return errors.New("no 'prometheus' label found")
	}

	// Step 2: get the corresponding AbsencePrometheusRule if it exists. We do this in
	// advance so that we can get suitable defaults for tier and service labels in the
	// next step.
	existingAbsencePrometheusRule := false
	absencePromRule, err := r.getExistingAbsencePrometheusRule(ctx, namespace, promServer)
	switch {
	case err == nil:
		existingAbsencePrometheusRule = true
	case apierrors.IsNotFound(err):
		absencePromRule = r.newAbsencePrometheusRule(namespace, promServer)
	default:
		// This could have been caused by a temporary network failure, or any
		// other transient reason.
		return err
	}

	// Step 3: get defaults for tier and service labels and add them to the AbsencePrometheusRule.
	labelOpts := LabelOpts{Keep: r.KeepLabel}
	if keepCCloudLabels(labelOpts.Keep) {
		// If the PrometheusRule has support group, tier, and service labels then use those as the defaults.
		labelOpts.DefaultSupportGroup = promRuleLabels[LabelCCloudSupportGroup]
		labelOpts.DefaultTier = promRuleLabels[LabelTier]
		labelOpts.DefaultService = promRuleLabels[LabelCCloudService]
		// If no labels are defined then we try to find the defaults using different
		// strategies in labelOptsWithCCloudDefaults().
		if labelOpts.DefaultSupportGroup == "" || labelOpts.DefaultService == "" {
			opts := r.labelOptsWithCCloudDefaults(ctx, absencePromRule)
			if opts != nil {
				labelOpts = *opts
			}
		}

		// TODO: this is here for backwards compatibility with old CCloud label format and
		// will be removed in a later version.
		if labelOpts.DefaultTier == "" || labelOpts.DefaultService == "" {
			opts := r.labelOptsWithDefaultTierAndService(ctx, absencePromRule)
			if opts != nil {
				labelOpts.DefaultTier = opts.DefaultTier
				// Service label from labelOptsWithCCloudDefaults() takes precedence over
				// labelOptsWithDefaultTierAndService().
				if labelOpts.DefaultService == "" {
					labelOpts.DefaultService = opts.DefaultService
				}
			}
		}

		// Update the defaults for the AbsencePrometheusRule in case they might've
		// changed.
		// New CCloud format.
		absencePromRule.Labels[LabelCCloudSupportGroup] = labelOpts.DefaultSupportGroup
		absencePromRule.Labels[LabelCCloudService] = labelOpts.DefaultService
		// Old CCloud format.
		absencePromRule.Labels[LabelTier] = labelOpts.DefaultTier
		absencePromRule.Labels[LabelService] = labelOpts.DefaultService
	}

	// Step 4: parse RuleGroups and generate corresponding absence alert rules.
	absenceRuleGroups, err := ParseRuleGroups(promRule.Spec.Groups, promRuleName, labelOpts)
	if err != nil {
		return err
	}

	// Step 5: we clean up orphaned absence alert rules from the AbsencePrometheusRule in
	// case no absence alert rules were generated.
	// This can happen when changes have been made to alert rules that result in no absent
	// alerts. E.g. absent() or the 'no_alert_on_absence' label was used.
	if len(absenceRuleGroups) == 0 {
		if existingAbsencePrometheusRule {
			key := types.NamespacedName{Namespace: namespace, Name: promRuleName}
			return r.cleanUpOrphanedAbsenceAlertRules(ctx, key, promServer)
		}
		return nil
	}

	// Step 6. log an error in case we couldn't find defaults for tier and service. We log
	// these errors after Step 4 and 5 to avoid unnecessary logging in case the
	// aforementioned steps result in no change.
	if keepCCloudLabels(labelOpts.Keep) {
		if labelOpts.DefaultSupportGroup == "" {
			log.Info("could not find a default value for 'support_group' label")
		}
		if labelOpts.DefaultTier == "" {
			log.Info("could not find a default value for 'tier' label")
		}
		if labelOpts.DefaultService == "" {
			log.Info("could not find a default value for 'service' label")
		}
	}

	// Step 7: if it's an existing AbsencePrometheusRule then update otherwise create a new resource.
	if existingAbsencePrometheusRule {
		existingRuleGroups := absencePromRule.Spec.Groups
		result := mergeAbsenceRuleGroups(existingRuleGroups, absenceRuleGroups)
		if reflect.DeepEqual(existingRuleGroups, result) {
			return nil
		}
		absencePromRule.Spec.Groups = result
		return r.updateAbsencePrometheusRule(ctx, absencePromRule)
	}
	absencePromRule.Spec.Groups = absenceRuleGroups
	return r.createAbsencePrometheusRule(ctx, absencePromRule)
}

// mergeAbsenceRuleGroups merges existing and newly generated AbsenceRuleGroups. If the
// same AbsenceRuleGroup exists in both 'existing' and 'new' then the newer one will be
// used.
func mergeAbsenceRuleGroups(existingRuleGroups, newRuleGroups []monitoringv1.RuleGroup) []monitoringv1.RuleGroup {
	var result []monitoringv1.RuleGroup
	added := make(map[string]bool)

OuterLoop:
	for _, oldG := range existingRuleGroups {
		for _, newG := range newRuleGroups {
			if oldG.Name == newG.Name {
				// Add the new updated RuleGroup.
				result = append(result, newG)
				added[newG.Name] = true
				continue OuterLoop
			}
		}
		// This RuleGroup should be carried over as is.
		result = append(result, oldG)
	}

	// Add the pending rule groups.
	for _, g := range newRuleGroups {
		if !added[g.Name] {
			result = append(result, g)
		}
	}
	return result
}
