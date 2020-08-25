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

// getPromRulefromAbsentRuleGroup takes the name of a RuleGroup name that holds
// absent alerts and returns the name of the origin PrometheusRule that holds
// the corresponding alert definitions. An empty string is returned if the name
// can't be determined.
//
// Absent alert RuleGroups names have the format:
//   originPrometheusRuleName/RuleGroupName
func getPromRulefromAbsentRuleGroup(group string) string {
	sL := strings.Split(group, "/")
	if len(sL) != 2 {
		return ""
	}
	return sL[0]
}

// cleanUpOrphanedAbsentAlertsNamespace deletes orphaned absent alerts across a
// cluster.
func (c *Controller) cleanUpOrphanedAbsentAlertsCluster() error {
	// Get names of all PrometheusRules that exist in the informer's cache:
	//   map of namespace to map[promRuleName]bool
	promRules := make(map[string]map[string]bool)
	objs := c.promRuleInformer.GetStore().List()
	for _, v := range objs {
		pr, ok := v.(*monitoringv1.PrometheusRule)
		if !ok {
			continue
		}
		ns := pr.GetNamespace()
		if _, ok = promRules[ns]; !ok {
			promRules[ns] = make(map[string]bool)
		}
		promRules[ns][pr.GetName()] = true
	}

	for namespace, promRuleNames := range promRules {
		// Get all absentPrometheusRules for this namespace.
		prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
			List(context.Background(), metav1.ListOptions{LabelSelector: labelOperatorManagedBy})
		if err != nil {
			return errors.Wrap(err, "could not list AbsentPrometheusRules")
		}

		for _, pr := range prList.Items {
			// Check if there are any alerts in this absentPrometheusRule that
			// don't belong to any PrometheusRule in promRuleNames.
			//
			// cleanup map is used because there could be multiple RuleGroups
			// that contain absent alerts concerning a single PrometheusRule
			// therefore we check all the groups before doing any cleanup.
			cleanup := make(map[string]struct{})
			for _, g := range pr.Spec.Groups {
				n := getPromRulefromAbsentRuleGroup(g.Name)
				if n != "" && !promRuleNames[n] {
					cleanup[n] = struct{}{}
				}
			}

			aPR := &absentPrometheusRule{PrometheusRule: pr}
			for n := range cleanup {
				if err := c.cleanUpOrphanedAbsentAlerts(n, aPR); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// cleanUpOrphanedAbsentAlertsNamespace deletes orphaned absent alerts
// concerning a specific PrometheusRule from a namespace.
//
// This is used when we don't know the prometheus server name of the
// PrometheusRule so we list all the AbsentPrometheusRules in a namespace and
// find the one that has the corresponding absent alerts.
func (c *Controller) cleanUpOrphanedAbsentAlertsNamespace(promRuleName, namespace string) error {
	prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelOperatorManagedBy})
	if err != nil {
		return errors.Wrap(err, "could not list AbsentPrometheusRules")
	}

	for _, pr := range prList.Items {
		for _, g := range pr.Spec.Groups {
			n := getPromRulefromAbsentRuleGroup(g.Name)
			if n != "" && n == promRuleName {
				aPR := &absentPrometheusRule{PrometheusRule: pr}
				err = c.cleanUpOrphanedAbsentAlerts(promRuleName, aPR)
				break
			}
		}
	}
	return err
}

// cleanUpOrphanedAbsentAlerts deletes orphaned absent alerts concerning a
// specific PrometheusRule from a specific absentPrometheusRule.
func (c *Controller) cleanUpOrphanedAbsentAlerts(promRuleName string, absentPromRule *absentPrometheusRule) error {
	old := absentPromRule.Spec.Groups
	new := make([]monitoringv1.RuleGroup, 0, len(old))
	for _, g := range old {
		n := getPromRulefromAbsentRuleGroup(g.Name)
		if n != "" && n == promRuleName {
			continue
		}
		new = append(new, g)
	}
	if reflect.DeepEqual(old, new) {
		return nil
	}

	var err error
	absentPromRule.Spec.Groups = new
	if len(absentPromRule.Spec.Groups) == 0 {
		err = c.promClientset.MonitoringV1().PrometheusRules(absentPromRule.Namespace).
			Delete(context.Background(), absentPromRule.Name, metav1.DeleteOptions{})
		if err == nil {
			c.logger.Debug("msg", "successfully deleted orphaned AbsentPrometheusRule",
				"key", fmt.Sprintf("%s/%s", absentPromRule.Namespace, absentPromRule.Name))
		}
	} else {
		_, err = c.promClientset.MonitoringV1().PrometheusRules(absentPromRule.Namespace).
			Update(context.Background(), absentPromRule.PrometheusRule, metav1.UpdateOptions{})
		if err == nil {
			c.logger.Debug("msg", "successfully cleaned up orphaned absent metric alert rules",
				"key", fmt.Sprintf("%s/%s", absentPromRule.Namespace, absentPromRule.Name))
		}
	}
	if err != nil {
		return errors.Wrap(err, "could not clean up orphaned absent metric alert rules")
	}

	return nil
}
