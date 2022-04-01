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
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promlabels "github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// metricNameExtractor is used to walk through a PromQL expression and extract
// time series (i.e. metric) names.
type metricNameExtractor struct {
	logger logr.Logger

	// expr is the PromQL expression that the metricNameExtractor is working on.
	expr string

	// This map contains metric names that were extracted from a promql.Node.
	// We only use the keys of the map and never depend on the presence of an
	// element in the map nor its value therefore we use an empty struct instead
	// of a bool.
	found map[string]struct{}
}

// Visit implements the parser.Visitor interface.
func (mex *metricNameExtractor) Visit(node parser.Node, path []parser.Node) (parser.Visitor, error) {
	vs, ok := node.(*parser.VectorSelector)
	if !ok {
		return mex, nil
	}

	err := errors.New("error while parsing PromQL query")
	name := vs.Name
	if name == "" {
		// Check if the VectorSelector uses label matching against the 'name'
		// label.
		for _, v := range vs.LabelMatchers {
			if v.Name != "__name__" {
				continue
			}

			switch v.Type {
			case promlabels.MatchEqual, promlabels.MatchNotEqual:
				name = v.Value
			case promlabels.MatchRegexp, promlabels.MatchNotRegexp:
				// Currently, we don't create absence alerts for regex name
				// label matching.
				// However, there are cases where some alert expressions use
				// regexp matching even where an equality would suffice.
				// E.g.:
				//   {__name__=~"http_requests_total"}
				rx, err := regexp.Compile(v.Value)
				if err != nil {
					// We do not return on error here so that any subsequent
					// VectorSelector(s) get a chance to be processed.
					mex.logger.Error(err, fmt.Sprintf("could not compile regex '%s'", v.Value),
						"expr", mex.expr)
					continue
				}
				if rx.MatchString(v.Value) {
					name = v.Value
				}
			}
		}
	}
	if name == "" {
		mex.logger.Error(err, fmt.Sprintf("could not find metric name for VectorSelector '%s'", vs.String()),
			"expr", mex.expr)
		return mex, nil
	}

	switch {
	case strings.Contains(mex.expr, fmt.Sprintf("absent(%s", name)):
		// Skip this metric if the there is already an
		// absent function for it in the original expression.
	case name == "up":
		// Skip "up" metric, it is automatically injected by
		// Prometheus to describe Prometheus scraping jobs.
	default:
		mex.found[name] = struct{}{}
	}
	return mex, nil
}

// absenceRuleGroupName returns the name of the RuleGroup that holds absence alert rules
// for a specific RuleGroup in a specific PrometheusRule.
func absenceRuleGroupName(promRule, ruleGroup string) string {
	return fmt.Sprintf("%s/%s", promRule, ruleGroup)
}

// promRulefromAbsenceRuleGroupName takes the name of a RuleGroup that holds absence alert
// rules and returns the name of the corresponding PrometheusRule that holds the actual
// alert definitions. An empty string is returned if the name can't be determined.
func promRulefromAbsenceRuleGroupName(ruleGroup string) string {
	sL := strings.Split(ruleGroup, "/")
	if len(sL) != 2 {
		return ""
	}
	return sL[0]
}

type ruleGroupParseError struct {
	cause error
}

// Error implements the error interface.
func (e *ruleGroupParseError) Error() string {
	return e.cause.Error()
}

// parseRuleGroups takes a slice of RuleGroup that has alert rules and returns
// a new slice of RuleGroup that has the corresponding absence alert rules.
//
// The original tier and service labels from the alert rules will be carried over to
// the corresponding absence alerts unless templating (i.e. $labels) was used
// for these labels in which case the provided default tier and service will be
// used.
//
// The rule group names for the absence alerts have the format:
//   promRuleName/originalGroupName.
func parseRuleGroups(in []monitoringv1.RuleGroup, promRuleName string, opts LabelOpts) ([]monitoringv1.RuleGroup, error) {
	out := make([]monitoringv1.RuleGroup, 0, len(in))
	for _, g := range in {
		var absenceAlertRules []monitoringv1.Rule
		for _, r := range g.Rules {
			// Do not parse recording rules.
			if r.Record != "" {
				continue
			}
			// Do not parse alert rule if it has the no_alert_on_absence label.
			if r.Labels != nil && parseBool(r.Labels[labelNoAlertOnAbsence]) {
				continue
			}
			rules, err := GenerateAbsenceAlertRules(r, opts)
			if err != nil {
				return nil, &ruleGroupParseError{cause: err}
			}
			if len(rules) > 0 {
				absenceAlertRules = append(absenceAlertRules, rules...)
			}
		}

		if len(absenceAlertRules) > 0 {
			out = append(out, monitoringv1.RuleGroup{
				Name:  absenceRuleGroupName(promRuleName, g.Name),
				Rules: absenceAlertRules,
			})
		}
	}
	return out, nil
}

// GenerateAbsenceAlertRules generates the corresponding absence alert rules for a given
// Rule. Since an alert expression can reference multiple time series therefore a slice of
// []monitoringv1.Rule is returned as multiple (one for each time series) absence alert
// rules would be generated.
func GenerateAbsenceAlertRules(in monitoringv1.Rule, opts LabelOpts) ([]monitoringv1.Rule, error) {
	exprStr := in.Expr.String()
	mex := &metricNameExtractor{
		// logger: c.logger, // TODO
		expr:  exprStr,
		found: map[string]struct{}{},
	}
	exprNode, err := parser.ParseExpr(exprStr)
	if err == nil {
		err = parser.Walk(mex, exprNode, nil)
	}
	if err != nil {
		// TODO: remove newline characters from expression.
		// The returned error has the expression at the end because
		// it could contain newline chracters.
		return nil, fmt.Errorf("could not parse rule expression: %s: %s", err.Error(), exprStr)
	}
	if len(mex.found) == 0 {
		return nil, nil
	}

	// Default labels.
	absenceRuleLabels := map[string]string{
		"context":  "absent-metrics",
		"severity": "info",
	}

	// Retain labels from the original alert rule.
	if ruleLabels := in.Labels; ruleLabels != nil {
		for k := range opts.Keep {
			v := ruleLabels[k]
			emptyOrTmplVal := (v == "" || strings.Contains(v, "$labels"))
			if k == LabelTier && emptyOrTmplVal {
				v = opts.DefaultTier
			}
			if k == LabelService && emptyOrTmplVal {
				v = opts.DefaultService
			}
			if v != "" {
				absenceRuleLabels[k] = v
			}
		}
	}

	// Sort metric names alphabetically for consistent test results.
	metrics := make([]string, 0, len(mex.found))
	for k := range mex.found {
		metrics = append(metrics, k)
	}
	sort.Strings(metrics)

	out := make([]monitoringv1.Rule, 0, len(metrics))
	for _, m := range metrics {
		// Generate an alert name from metric name. Example:
		//   network:tis_a_metric:rate5m -> AbsentTierServiceNetworkTisAMetricRate5m
		words := []string{"absent", absenceRuleLabels[LabelTier], absenceRuleLabels[LabelService]}
		sL1 := strings.Split(m, ":")
		for _, v := range sL1 {
			sL2 := strings.Split(v, "_")
			words = append(words, sL2...)
		}
		// Avoid name stuttering
		var alertName string
		var prevW string
		for _, w := range words {
			w := strings.ToLower(w) // convert to lowercase for comparison
			if w != prevW {
				alertName += cases.Title(language.English).String(w)
				prevW = w
			}
		}

		// TODO: remove the link from description and add a 'playbook' label,
		// when our upstream solution gets the ability to process hardcoded
		// links in the 'playbook' label.
		ann := map[string]string{
			"summary": fmt.Sprintf("missing %s", m),
			"description": fmt.Sprintf(
				"The metric '%s' is missing. '%s' alert using it may not fire as intended. "+
					"See <https://github.com/sapcc/absent-metrics-operator/blob/master/doc/playbook.md|the operator playbook>.",
				m, in.Alert,
			),
		}

		out = append(out, monitoringv1.Rule{
			Alert:       alertName,
			Expr:        intstr.FromString(fmt.Sprintf("absent(%s)", m)),
			For:         "10m",
			Labels:      absenceRuleLabels,
			Annotations: ann,
		})
	}

	return out, nil
}
