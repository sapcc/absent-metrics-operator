// Copyright 2024 SAP SE
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("AbsencePrometheusRule", func() {
	pr := &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foobar.alerts",
			Namespace: "outerspace",
			Labels: map[string]string{
				"prometheus":   "openstack",
				"thanos-ruler": "titan",
			},
		},
	}

	DescribeTable("Name generation",
		func(tmplStr, expected string, shouldFail bool) {
			gen, err := CreateAbsencePromRuleNameGenerator(tmplStr)
			Expect(err).ToNot(HaveOccurred())
			actual, err := gen(pr)
			if shouldFail {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).ToNot(HaveOccurred())
			Expect(actual).To(Equal(expected))
		},
		Entry("name that uses the original name",
			`{{ .metadata.name }}`,
			"foobar.alerts"+absencePromRuleNameSuffix,
			false,
		),
		Entry("name that uses the namespace",
			`{{ .metadata.namespace }}`,
			"outerspace"+absencePromRuleNameSuffix,
			false,
		),
		Entry("name that uses the original name and namespace",
			`{{ .metadata.name }}-{{ .metadata.namespace }}`,
			"foobar.alerts-outerspace"+absencePromRuleNameSuffix,
			false,
		),
		Entry("name with prometheus label",
			`{{ .metadata.labels.prometheus }}`,
			"openstack"+absencePromRuleNameSuffix,
			false,
		),
		Entry("name with thanos-ruler label if it exists",
			DefaultAbsencePromRuleNameTemplate,
			"titan"+absencePromRuleNameSuffix,
			false,
		),
		Entry("name that references nonexistent metadata",
			`{{ .metadata.doesntexist }}`,
			"",
			true,
		),
	)
})
