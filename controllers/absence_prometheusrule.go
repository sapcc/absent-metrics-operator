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
	if err := r.Create(ctx, absencePromRule); err != nil {
		return err
	}

	r.Log.V(logLevelDebug).Info("successfully created AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

func (r *PrometheusRuleReconciler) patchAbsencePrometheusRule(
	ctx context.Context,
	absencePromRule,
	unmodifiedAbsencePromRule *monitoringv1.PrometheusRule,
) error {

	sortRuleGroups(absencePromRule)
	updateAnnotationTime(absencePromRule)
	if err := r.Patch(ctx, absencePromRule, client.MergeFrom(unmodifiedAbsencePromRule)); err != nil {
		return err
	}

	r.Log.V(logLevelDebug).Info("successfully updated AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

func (r *PrometheusRuleReconciler) deleteAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	if err := r.Delete(ctx, absencePromRule); err != nil {
		return err
	}

	r.Log.V(logLevelDebug).Info("successfully deleted AbsencePrometheusRule",
		"AbsencePrometheusRule", fmt.Sprintf("%s/%s", absencePromRule.GetNamespace(), absencePromRule.GetName()))
	return nil
}

var errCorrespondingAbsencePromRuleNotExists = errors.New("corresponding AbsencePrometheusRule for clean up does not exist")

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
		return errCorrespondingAbsencePromRuleNotExists
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
	if len(newRuleGroups) == 0 {
		return r.deleteAbsencePrometheusRule(ctx, aPRToClean)
	}
	unmodified := aPRToClean.DeepCopy()
	aPRToClean.Spec.Groups = newRuleGroups
	return r.patchAbsencePrometheusRule(ctx, aPRToClean, unmodified)
}

// cleanUpAbsencePrometheusRule checks an AbsencePrometheusRule to see if it contains
// absence alert rules for a PrometheusRule that no longer exists or for a resource that
// has the 'absent-metrics-operator/disable' label. If such rules are found then they are
// deleted.
func (r *PrometheusRuleReconciler) cleanUpAbsencePrometheusRule(ctx context.Context, absencePromRule *monitoringv1.PrometheusRule) error {
	// Step 1: get names of all PrometheusRule resources in this namespace for the
	// concerning Prometheus server.
	var listOpts client.ListOptions
	client.InNamespace(absencePromRule.GetNamespace()).ApplyToList(&listOpts)
	client.MatchingLabels{
		labelPrometheusServer: absencePromRule.Labels[labelPrometheusServer],
	}.ApplyToList(&listOpts)
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
	if len(newRuleGroups) == 0 {
		return r.deleteAbsencePrometheusRule(ctx, absencePromRule)
	}
	unmodified := absencePromRule.DeepCopy()
	absencePromRule.Spec.Groups = newRuleGroups
	return r.patchAbsencePrometheusRule(ctx, absencePromRule, unmodified)
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

	// Step 2: get the corresponding AbsencePrometheusRule if it exists.
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

	unmodifiedAbsencePromRule := absencePromRule.DeepCopy()

	// Remove resource level tier, service, and support-group labels from existing PrometheusRule
	// objects created by the operator.
	// TODO: remove this after August 2024, by then the labels should have been removed from all
	// PrometheusRules created by the operator.
	if r.KeepLabel[LabelSupportGroup] && r.KeepLabel[LabelTier] && r.KeepLabel[LabelService] {
		delete(absencePromRule.Labels, LabelCCloudSupportGroup)
		delete(absencePromRule.Labels, LabelCCloudService)
		delete(absencePromRule.Labels, LabelService)
		delete(absencePromRule.Labels, LabelTier)
	}

	// Step 3: parse RuleGroups and generate corresponding absence alert rules.
	absenceRuleGroups, err := ParseRuleGroups(log, promRule.Spec.Groups, promRuleName, r.KeepLabel)
	if err != nil {
		return err
	}

	// Step 4: we clean up orphaned absence alert rules from the AbsencePrometheusRule in
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

	// Step 5: if it's an existing AbsencePrometheusRule then update otherwise create a new resource.
	if existingAbsencePrometheusRule {
		existingRuleGroups := absencePromRule.Spec.Groups
		result := mergeAbsenceRuleGroups(existingRuleGroups, absenceRuleGroups)
		if reflect.DeepEqual(unmodifiedAbsencePromRule.GetLabels(), absencePromRule.GetLabels()) &&
			reflect.DeepEqual(existingRuleGroups, result) {
			return nil
		}
		absencePromRule.Spec.Groups = result
		return r.patchAbsencePrometheusRule(ctx, absencePromRule, unmodifiedAbsencePromRule)
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
