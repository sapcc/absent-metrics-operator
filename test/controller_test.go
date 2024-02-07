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

package test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/sapcc/absent-metrics-operator/controllers"
)

var _ = Describe("Controller", func() {
	// Note: Throughout the individual tests we will use two terms:
	//   *PromRule(s)*: this refers to a PrometheusRule resource(s) that contains alert rules.
	//   *AbsentPromRule(s)*: this refers to the PrometheusRule resource(s) created by the
	//   controller that contains the corresponding absent alert rules for a
	//   PrometheusRule resource.
	//
	// The fixtures/start-data contains four PromRules which should have already been
	// added to our test cluster.
	// They are called (namespace/name):
	// 	 - resmgmt/kubernetes-keppel.alerts
	// 	 - resmgmt/openstack-limes-api.alerts
	// 	 - resmgmt/openstack-limes-roleassign.alerts
	// 	 - swift/openstack-swift.alerts

	var (
		k8sAbsentPRName = controllers.AbsencePrometheusRuleName("kubernetes")
		osAbsentPRName  = controllers.AbsencePrometheusRuleName("openstack")

		swiftNs   = "swift"
		resmgmtNs = "resmgmt"

		swiftOSAbsentPromRule    = getFixture("swift_openstack_absent_metric_alert_rules.yaml")
		resmgmtK8sAbsentPromRule = getFixture("resmgmt_kubernetes_absent_metric_alert_rules.yaml")
		resmgmtOSAbsentPromRule  = getFixture("resmgmt_openstack_absent_metrics_alert_rules.yaml")
	)

	Describe("Create", func() {
		Context("during initial run", func() {
			It("should create "+k8sAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				expected := resmgmtK8sAbsentPromRule
				actual, err := getPromRule(newObjKey(resmgmtNs, k8sAbsentPRName))
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Annotations).To(Equal(expected.Annotations))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})

			It("should create "+osAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				expected := resmgmtOSAbsentPromRule
				actual, err := getPromRule(newObjKey(resmgmtNs, osAbsentPRName))
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Annotations).To(Equal(expected.Annotations))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})

			It("should not create "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				_, err := getPromRule(newObjKey(swiftNs, osAbsentPRName))
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})

		Context("after removing absent-metrics-operator/disable label", func() {
			It("should create "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				pr, err := getPromRule(newObjKey(swiftNs, "openstack-swift.alerts"))
				Expect(err).ToNot(HaveOccurred())

				// Update the "disable" label on the PromRule.
				pr.Labels["absent-metrics-operator/disable"] = "false"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Check that the corresponding AbsentPromRule was created.
				waitForControllerToProcess()
				expected := swiftOSAbsentPromRule
				actual, err := getPromRule(newObjKey(swiftNs, osAbsentPRName))
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Annotations).To(Equal(expected.Annotations))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})
	})

	Describe("Update", func() {
		objKey := newObjKey(swiftNs, "openstack-swift.alerts")
		prObjKey := newObjKey(swiftNs, osAbsentPRName)
		labelOpts := controllers.LabelOpts{
			DefaultSupportGroup: "not-containers",
			DefaultTier:         "os",
			DefaultService:      "swift",
			Keep:                keepLabel,
		}
		fooBar := "foo_bar"
		barFoo := "bar_foo"

		Context("by adding a new alert rule", func() {
			It("should update "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				pr, err := getPromRule(objKey)
				Expect(err).ToNot(HaveOccurred())

				// Add a new alert rule.
				rule := createMockRule(fooBar)
				pr.Spec.Groups[0].Rules = append(pr.Spec.Groups[0].Rules, rule)
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Generate the corresponding absent alert rules.
				expected, err := controllers.ParseRuleGroups(logger, pr.Spec.Groups, pr.GetName(), labelOpts)
				Expect(err).ToNot(HaveOccurred())

				// Get the updated AbsentPromRule from the server and check if it has the
				// corresponding absent alert rule.
				waitForControllerToProcess()
				absentPR, err := getPromRule(prObjKey)
				Expect(err).ToNot(HaveOccurred())
				actual := absentPR.Spec.Groups
				Expect(actual).To(Equal(expected))
			})
		})

		Context("by updating an existing alert rule", func() {
			It("should update "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				pr, err := getPromRule(objKey)
				Expect(err).ToNot(HaveOccurred())

				// Update an existing alert rule. Replace alert that has foo_bar metric
				// with bar_foo.
			OuterLoop:
				for gIdx, g := range pr.Spec.Groups {
					for rIdx, r := range g.Rules {
						if strings.Contains(r.Expr.String(), fooBar) {
							pr.Spec.Groups[gIdx].Rules[rIdx] = createMockRule(barFoo)
							break OuterLoop
						}
					}
				}
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Generate the corresponding absent alert rules.
				expected, err := controllers.ParseRuleGroups(logger, pr.Spec.Groups, pr.GetName(), labelOpts)
				Expect(err).ToNot(HaveOccurred())

				// Get the updated AbsentPromRule from the server and check if the
				// corresponding absent alert rule has been updated.
				waitForControllerToProcess()
				absentPR, err := getPromRule(prObjKey)
				Expect(err).ToNot(HaveOccurred())
				actual := absentPR.Spec.Groups
				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("Cleanup", func() {
		Context("when a PrometheusRule is deleted", func() {
			It("should delete "+k8sAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				err := deletePromRule(newObjKey(resmgmtNs, "kubernetes-keppel.alerts"))
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				_, err = getPromRule(newObjKey(resmgmtNs, k8sAbsentPRName))
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("should delete orphaned absent alert rules from "+osAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				// AbsentPromRule 'openstack-absent-metric-alert-rules' in 'resmgmt'
				// namespace holds the aggregate absent alert rules for both
				// 'openstack-limes-api.alerts' and 'openstack-limes-roleassign.alerts'
				// PromRules.
				limesRolePRName := "openstack-limes-roleassign.alerts"
				err := deletePromRule(newObjKey(resmgmtNs, limesRolePRName))
				Expect(err).ToNot(HaveOccurred())

				expected := make([]monitoringv1.RuleGroup, 0, len(resmgmtOSAbsentPromRule.Spec.Groups)-1)
				for _, g := range resmgmtOSAbsentPromRule.Spec.Groups {
					if !strings.Contains(g.Name, limesRolePRName) {
						expected = append(expected, g)
					}
				}

				// Deleting one PromRule should only result in cleanup of its
				// corresponding absent alert rules from AbsentPromRule instead of
				// deleting the AbsentPromRule itself.
				waitForControllerToProcess()
				actual, err := getPromRule(newObjKey(resmgmtNs, osAbsentPRName))
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec.Groups).To(Equal(expected))
			})
		})

		Context("when a PrometheusRule has no alert rules", func() {
			It("should delete "+osAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				// We will remove all alert rules from a PromRule which should result in
				// the deletion of its corresponding AbsentPromRule since no absent alert
				// rules will be generated and the resulting AbsentPromRule would be
				// empty.
				pr, err := getPromRule(newObjKey(resmgmtNs, "openstack-limes-api.alerts"))
				Expect(err).ToNot(HaveOccurred())
				pr.Spec.Groups = []monitoringv1.RuleGroup{}
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				_, err = getPromRule(newObjKey("resmgmt", osAbsentPRName))
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Describe("Disabling", func() {
		objKey := newObjKey(swiftNs, "openstack-swift.alerts")
		prObjKey := newObjKey(swiftNs, osAbsentPRName)

		Context("for a specific alert rule", func() {
			It("should delete the corresponding absent alert rule from "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Add the 'no_alert_on_absence' label to the first rule of first group in
				// the PromRule. This should result in the deletion of the corresponding
				// absent alert rule from the AbsentPromRule.
				pr, err := getPromRule(objKey)
				Expect(err).ToNot(HaveOccurred())
				pr.Spec.Groups[0].Rules[0].Labels["no_alert_on_absence"] = "true"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Generate the corresponding absent alert rules.
				expected, err := controllers.ParseRuleGroups(logger, pr.Spec.Groups, pr.GetName(), controllers.LabelOpts{
					DefaultSupportGroup: "not-containers",
					DefaultTier:         "os",
					DefaultService:      "swift",
					Keep:                keepLabel,
				})
				Expect(err).ToNot(HaveOccurred())

				// Check that the corresponding absent alert rule was removed.
				waitForControllerToProcess()
				aPR, err := getPromRule(prObjKey)
				Expect(err).ToNot(HaveOccurred())
				Expect(aPR.Spec.Groups).To(Equal(expected))
			})
		})

		Context("for an entire PrometheusRule", func() {
			It("should delete "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Add the 'absent-metrics-operator/disable' label to the PromRule. This
				// should result in the deletion of the corresponding AbsentPromRule.
				pr, err := getPromRule(objKey)
				Expect(err).ToNot(HaveOccurred())
				pr.Labels["absent-metrics-operator/disable"] = "true"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				_, err = getPromRule(prObjKey)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})
		})
	})

	Describe("Orphaned gauge metrics", func() {
		It("should be cleaned up", func() {
			method := http.MethodGet
			path := "/metrics"
			var requestBody io.Reader
			request := httptest.NewRequest(method, path, requestBody)

			recorder := httptest.NewRecorder()
			handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
			handler.ServeHTTP(recorder, request)
			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(http.StatusOK))

			// Write actual content to file to make it easy to copy the
			// computed result over to the fixture path when a new test is
			// added or an existing one is modified.
			fixturePath, err := filepath.Abs("fixtures/metrics.prom")
			Expect(err).ToNot(HaveOccurred())
			actualBytes, err := io.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())
			actualPath := fixturePath + ".actual"
			err = os.WriteFile(actualPath, actualBytes, 0o600)
			Expect(err).ToNot(HaveOccurred())

			cmd := exec.Command("diff", "-u", fixturePath, actualPath)
			cmd.Stdin = nil
			cmd.Stdout = GinkgoWriter
			cmd.Stderr = os.Stderr
			if err = cmd.Run(); err != nil {
				Fail(fmt.Sprintf("%s %s: body does not match", method, path))
			}
		})
	})
})

///////////////////////////////////////////////////////////////////////////////
// Helper functions

// Wait for controller to resync and complete its processing.
var waitForControllerToProcess = func() { time.Sleep(500 * time.Millisecond) }

func newObjKey(namespace, name string) client.ObjectKey {
	return client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}
}

func createMockRule(metric string) monitoringv1.Rule {
	duration := monitoringv1.Duration("5m")
	return monitoringv1.Rule{
		Alert: cases.Title(language.English).String(metric),
		Expr:  intstr.FromString(metric + " > 0"),
		For:   &duration,
		Labels: map[string]string{
			"tier":    "tier",
			"service": "service",
		},
	}
}

func getFixture(name string) monitoringv1.PrometheusRule {
	b, err := os.ReadFile(filepath.Join("fixtures", name))
	Expect(err).ToNot(HaveOccurred())

	var pr monitoringv1.PrometheusRule
	err = yaml.Unmarshal(b, &pr)
	Expect(err).ToNot(HaveOccurred())

	return pr
}

func getPromRule(key client.ObjectKey) (monitoringv1.PrometheusRule, error) {
	var pr monitoringv1.PrometheusRule
	err := k8sClient.Get(ctx, key, &pr)
	return pr, err
}

func deletePromRule(key client.ObjectKey) error {
	var pr monitoringv1.PrometheusRule
	pr.SetName(key.Name)
	pr.SetNamespace(key.Namespace)
	return k8sClient.Delete(ctx, &pr)
}
