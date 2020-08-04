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
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
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

	_, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		Create(context.Background(), pr, metav1.CreateOptions{})
	return err
}

// updateAbsentPrometheusRule takes a PrometheusRule and updates it with the
// provided slice of RuleGroup.
func (c *Controller) updateAbsentPrometheusRule(
	namespace string,
	absentPromRule *monitoringv1.PrometheusRule,
	rg []monitoringv1.RuleGroup) error {

	// The provided promRule is read-only, local cache from the Store.
	// We make a deep copy of the original object and modify that.
	pr := absentPromRule.DeepCopy()

	// Check if the promRule already has these rule groups.
	// Update if it does otherwise append.
	updated := make(map[string]bool)
	for _, g := range rg {
		for i, v := range pr.Spec.Groups {
			if g.Name == v.Name {
				pr.Spec.Groups[i] = g
				updated[g.Name] = true
			}
		}
	}
	new := make([]monitoringv1.RuleGroup, 0, len(rg)-len(updated)+len(pr.Spec.Groups))
	new = append(new, pr.Spec.Groups...)
	for _, g := range rg {
		if !updated[g.Name] {
			new = append(new, g)
		}
	}
	pr.Spec.Groups = new

	_, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		Update(context.Background(), pr, metav1.UpdateOptions{})
	return err
}

// deleteAbsentAlertRulesNamespace deletes absent alert rules concerning
// a specific PrometheusRule from all absent alert PrometheusRule resources
// across a namespace.
func (c *Controller) deleteAbsentAlertRulesNamespace(namespace, promRuleName string) error {
	prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelManagedBy})
	if err != nil {
		return fmt.Errorf("could not list absent alert PrometheusRules for '%s': %s", namespace, err.Error())
	}

	for _, pr := range prList.Items {
		if err := c.deleteAbsentAlertRules(namespace, promRuleName, pr); err != nil {
			return err
		}
	}

	return nil
}

// deleteAbsentAlertRules deletes absent alert rules concerning a specific
// PrometheusRule from a specific PrometheusRule.
func (c *Controller) deleteAbsentAlertRules(namespace, promRuleName string, absentPromRule *monitoringv1.PrometheusRule) error {
	// The provided promRule is read-only, local cache from the Store.
	// We make a deep copy of the original object and modify that.
	pr := absentPromRule.DeepCopy()

	var new []monitoringv1.RuleGroup
	for _, g := range pr.Spec.Groups {
		// The rule groups for absent alert PrometheusRules have the
		// format: promRuleName/ruleGroupName.
		if !strings.Contains(g.Name, promRuleName) {
			new = append(new, g)
		}
	}
	pr.Spec.Groups = new

	var err error
	if len(pr.Spec.Groups) == 0 {
		err = c.promClientset.MonitoringV1().PrometheusRules(namespace).
			Delete(context.Background(), pr.Name, metav1.DeleteOptions{})
	} else {
		_, err = c.promClientset.MonitoringV1().PrometheusRules(namespace).
			Update(context.Background(), pr, metav1.UpdateOptions{})
	}
	return err
}
