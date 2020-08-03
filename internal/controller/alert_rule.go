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
	"fmt"
	"strings"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/prometheus/promql"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
)

type metricNameExtractor struct {
	// This map contains metric names that were extracted from a promql.Node.
	// We only use the keys of the map and never depend on the presence of an
	// element in the map nor its value therefore an empty struct is better
	// than bool.
	found map[string]struct{}
}

func (mex *metricNameExtractor) Visit(node promql.Node) promql.Visitor {
	switch n := node.(type) {
	case *promql.MatrixSelector:
		mex.found[n.Name] = struct{}{}
	case *promql.VectorSelector:
		mex.found[n.Name] = struct{}{}
	}
	return mex
}

// parseRuleGroups takes a slice of RuleGroup that has alert rules and returns
// a new slice of RuleGroup that has the corresponding absent metric alert rules.
//
// The original tier and service labels of alert rules will be carried over to
// the corresponding absent alert rule unless templating was used (i.e. $labels)
// for these labels in which case the provided tier and service will be used.
//
// The rule group names have the format: promRuleName/originalGroupName.
func parseRuleGroups(promRuleName, tier, service string, in []monitoringv1.RuleGroup) ([]monitoringv1.RuleGroup, error) {
	out := make([]monitoringv1.RuleGroup, 0, len(in))
	for _, g := range in {
		var absentRules []monitoringv1.Rule
		for _, r := range g.Rules {
			exprStr := r.Expr.String()
			exprNode, err := promql.ParseExpr(exprStr)
			if err != nil {
				// The returned error has the expression in last because
				// it could contain newline chracters.
				return nil, fmt.Errorf("could not parse rule expression: %s: %s",
					err.Error(), r.Expr.String())
			}

			mex := &metricNameExtractor{found: map[string]struct{}{}}
			promql.Walk(mex, exprNode)
			if len(mex.found) == 0 {
				continue // to next rule
			}
			for k := range mex.found {
				switch {
				case strings.Contains(exprStr, fmt.Sprintf("absent(%s", k)):
					// Skip this metric if the there is already an
					// absent function for it in the original expression.
					continue
				case k == "up":
					// Skip "up" metric, it is automatically injected by
					// Prometheus to describe Prometheus scraping jobs.
					continue
				default:
					absentRules = append(absentRules, newAbsentRule(k, tier, service, r.Labels))
				}
			}
		}

		if len(absentRules) > 0 {
			out = append(out, monitoringv1.RuleGroup{
				Name:  fmt.Sprintf("%s/%s", promRuleName, g.Name),
				Rules: absentRules,
			})
		}
	}

	return out, nil
}

// newAbsentRule takes a metric name and labels and returns a corresponding
// absent metric rule.
func newAbsentRule(metric, tier, service string, originalLabels map[string]string) monitoringv1.Rule {
	// Carry over labels from the original alert
	if originalLabels != nil {
		if v, ok := originalLabels["tier"]; ok {
			if !strings.Contains(v, "$labels") {
				tier = v
			}
		}
		if v, ok := originalLabels["service"]; ok {
			if !strings.Contains(v, "$labels") {
				service = v
			}
		}
	}
	labels := map[string]string{
		"tier":     tier,
		"service":  service,
		"severity": "info",
	}

	// Generate an alert name from metric name:
	//   network:tis_a_metric:rate5m -> Absent("tier")("service")NetworkTisAMetricRate5m
	words := []string{"absent", tier, service}
	sL1 := strings.Split(metric, "_")
	for _, v := range sL1 {
		sL2 := strings.Split(v, ":")
		words = append(words, sL2...)
	}
	// Avoid name stuttering
	var alertName string
	var prev string
	for _, v := range words {
		l := strings.ToLower(v)
		if l != prev {
			prev = l
			alertName += strings.Title(l)
		}
	}

	annotations := map[string]string{
		"summary":     fmt.Sprintf("missing %s", metric),
		"description": fmt.Sprintf("The metric '%s' is missing", metric),
	}

	return monitoringv1.Rule{
		Alert:       alertName,
		Expr:        intstr.FromString(fmt.Sprintf("absent(%s)", metric)),
		For:         "10m",
		Labels:      labels,
		Annotations: annotations,
	}
}
