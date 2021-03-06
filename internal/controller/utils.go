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
	"strconv"
	"strings"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// getTierAndService returns the most common tier and service combination used
// in the alert definitions.
//
// See parseRuleGroups() for info on why we need this.
func getTierAndService(rg []monitoringv1.RuleGroup) (tier, service string) {
	// Map of tier to service to number of occurrences.
	count := make(map[string]map[string]int)
	for _, g := range rg {
		for _, r := range g.Rules {
			if r.Record != "" {
				continue
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

// mustParseBool is a wrapper for strconv.ParseBool() that returns false in
// case of an error.
func mustParseBool(str string) bool {
	v, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return v
}
