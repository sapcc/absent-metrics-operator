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

// cleanUpOrphanedAbsentAlertsNamespace deletes orphaned absent alerts
// concerning a specific PrometheusRule from a namespace.
func (c *Controller) cleanUpOrphanedAbsentAlertsNamespace(namespace, promRuleName string) error {
	prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		List(context.Background(), metav1.ListOptions{LabelSelector: labelOperatorManagedBy})
	if err != nil {
		return errors.Wrap(err, "could not list AbsentPrometheusRules")
	}

	for _, pr := range prList.Items {
		aPR := &absentPrometheusRule{PrometheusRule: pr}
		err := c.cleanUpOrphanedAbsentAlerts(promRuleName, aPR)
		if err != nil {
			return err
		}
	}

	return nil
}

// cleanUpOrphanedAbsentAlerts deletes orphaned absent alerts concerning a
// specific PrometheusRule from a specific absentPrometheusRule.
func (c *Controller) cleanUpOrphanedAbsentAlerts(promRuleName string, absentPromRule *absentPrometheusRule) error {
	old := absentPromRule.Spec.Groups
	new := make([]monitoringv1.RuleGroup, 0, len(old))
	for _, g := range old {
		// The rule group names for absentPrometheusRule have the format:
		// originPromRuleName/ruleGroupName.
		sL := strings.Split(g.Name, "/")
		if len(sL) > 0 && sL[0] == promRuleName {
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
