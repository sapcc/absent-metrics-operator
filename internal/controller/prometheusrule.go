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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createAbsentPrometheusRule creates a new PrometheusRule with the given
// RuleGroup and name for the given namespace and prometheus server.
func (c *Controller) createAbsentPrometheusRule(namespace, name, promServerName string, rg []monitoringv1.RuleGroup) error {
	// Add a label that identifies that this PrometheusRule is created
	// and managed by this operator.
	labels := map[string]string{
		"prometheus":   promServerName,
		"type":         "alerting-rules",
		labelManagedBy: "true",
	}
	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: rg,
		},
	}

	_, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).Create(context.Background(), pr, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "could not create new absent PrometheusRule")
	}

	c.logger.Info("msg", "successfully created new absent PrometheusRule",
		"key", fmt.Sprintf("%s/%s", namespace, name))
	return nil
}

// updateAbsentPrometheusRule takes a PrometheusRule and updates it with the
// provided slice of RuleGroup.
func (c *Controller) updateAbsentPrometheusRule(
	namespace string,
	absentPR *monitoringv1.PrometheusRule,
	rg []monitoringv1.RuleGroup) error {

	// Check if the absent PrometheusRule already has these rule groups.
	// Update if it does, otherwise append.
	old := absentPR.Spec.Groups
	var new []monitoringv1.RuleGroup
	updated := make(map[string]bool)
OuterLoop:
	for _, oldG := range old {
		for _, g := range rg {
			if oldG.Name == g.Name {
				// Add the new updated RuleGroup.
				new = append(new, g)
				updated[g.Name] = true
				continue OuterLoop
			}
		}
		// This RuleGroup should be carried over as is.
		new = append(new, oldG)
	}
	// Add the pending RuleGroups.
	for _, g := range rg {
		if !updated[g.Name] {
			new = append(new, g)
		}
	}

	// No need to update if old and new rule groups are exactly the same.
	if reflect.DeepEqual(old, new) {
		return nil
	}

	absentPR.Spec.Groups = new
	_, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).Update(context.Background(), absentPR, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrap(err, "could not update absent PrometheusRule")
	}

	c.logger.Info("msg", "successfully updated absent metric alert rules",
		"key", fmt.Sprintf("%s/%s", namespace, absentPR.Name))
	return nil
}

// deleteAbsentAlertRulesNamespace deletes absent alert rules concerning
// a specific PrometheusRule from all absent alert PrometheusRule resources
// across a namespace.
func (c *Controller) deleteAbsentAlertRulesNamespace(namespace, promRuleName string) error {
	prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelManagedBy})
	if err != nil {
		return errors.Wrap(err, "could not list absent PrometheusRules")
	}

	for _, pr := range prList.Items {
		if err := c.deleteAbsentAlertRules(namespace, promRuleName, pr); err != nil {
			return err
		}
	}

	return nil
}

// deleteAbsentAlertRules deletes absent alert rules concerning a specific
// PrometheusRule from a specific absent PrometheusRule.
func (c *Controller) deleteAbsentAlertRules(namespace, promRuleName string, absentPR *monitoringv1.PrometheusRule) error {
	old := absentPR.Spec.Groups
	var new []monitoringv1.RuleGroup
	for _, g := range old {
		// The rule group names for absent PrometheusRule have the format:
		// originPromRuleName/ruleGroupName.
		if !strings.Contains(g.Name, promRuleName) {
			new = append(new, g)
		}
	}
	if reflect.DeepEqual(old, new) {
		return nil
	}

	var err error
	absentPR.Spec.Groups = new
	if len(absentPR.Spec.Groups) == 0 {
		err = c.promClientset.MonitoringV1().PrometheusRules(namespace).Delete(context.Background(), absentPR.Name, metav1.DeleteOptions{})
		if err == nil {
			c.logger.Info("msg", "successfully deleted orphaned absent PrometheusRule",
				"key", fmt.Sprintf("%s/%s", namespace, absentPR.Name))
		}
	} else {
		_, err = c.promClientset.MonitoringV1().PrometheusRules(namespace).Update(context.Background(), absentPR, metav1.UpdateOptions{})
		if err == nil {
			c.logger.Info("msg", "successfully cleaned up orphaned absent metric alert rules",
				"key", fmt.Sprintf("%s/%s", namespace, absentPR.Name))
		}
	}
	if err != nil {
		return errors.Wrap(err, "could not clean up orphaned absent metric alert rules")
	}

	return nil
}
