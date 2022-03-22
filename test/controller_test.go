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
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghodss/yaml"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sapcc/absent-metrics-operator/internal/controller"
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
		ctx = context.Background()

		k8sAbsentPRName = controller.AbsentPrometheusRuleName("kubernetes")
		osAbsentPRName  = controller.AbsentPrometheusRuleName("openstack")

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
				var actual monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, newObjKey(resmgmtNs, k8sAbsentPRName), &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})

			It("should create "+osAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				expected := resmgmtOSAbsentPromRule
				var actual monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, newObjKey(resmgmtNs, osAbsentPRName), &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})

			It("should not create "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				var actual monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, newObjKey(swiftNs, osAbsentPRName), &actual)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})
		})

		Context("after removing absent-metrics-operator/disable label", func() {
			It("should create "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, newObjKey(swiftNs, "openstack-swift.alerts"), &pr)
				Expect(err).ToNot(HaveOccurred())

				// Update the "disable" label on the PromRule.
				pr.Labels["absent-metrics-operator/disable"] = "false"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Check that the corresponding AbsentPromRule was created.
				waitForControllerToProcess()
				expected := swiftOSAbsentPromRule
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, newObjKey(swiftNs, osAbsentPRName), &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})
	})

	Describe("Update", func() {
		objKey := newObjKey(swiftNs, "openstack-swift.alerts")
		prObjKey := newObjKey(swiftNs, osAbsentPRName)
		tier, service := "os", "swift"

		Context("by adding a new alert rule", func() {
			It("should update "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Add a new alert rule to the first group for easier lookup.
				rule := createMockRule("foo_bar")
				pr.Spec.Groups[0].Rules = append(pr.Spec.Groups[0].Rules, rule)
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Generate the corresponding absent alert rules.
				rL, err := c.ParseAlertRule(tier, service, rule)
				Expect(err).ToNot(HaveOccurred())
				expected := rL[0]

				// Get the updated AbsentPromRule from the server and check if it has the
				// corresponding absent alert rule.
				waitForControllerToProcess()
				var absentPR monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &absentPR)
				Expect(err).ToNot(HaveOccurred())
				rIdx := len(absentPR.Spec.Groups[0].Rules) - 1
				// The new absent alert rule should've been ended to the end of the first group.
				actual := absentPR.Spec.Groups[0].Rules[rIdx]
				Expect(actual).To(Equal(expected))
			})
		})

		Context("by updating an existing alert rule", func() {
			It("should update "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the concerning PromRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Update an existing alert rule.
				rule := createMockRule("bar_foo")
				pr.Spec.Groups[0].Rules[0] = rule // first rule of first group
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Generate the corresponding absent alert rule.
				rL, err := c.ParseAlertRule(tier, service, rule)
				Expect(err).ToNot(HaveOccurred())
				expected := rL[0]

				// Get the updated AbsentPromRule from the server and check if the
				// corresponding absent alert rule has been updated.
				waitForControllerToProcess()
				var absentPR monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &absentPR)
				Expect(err).ToNot(HaveOccurred())
				actual := absentPR.Spec.Groups[0].Rules[0]
				Expect(actual).To(Equal(expected))
			})
		})
	})

	Describe("Cleanup", func() {
		Context("when a PrometheusRule is deleted", func() {
			It("should delete "+k8sAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				var pr monitoringv1.PrometheusRule
				pr.Namespace = resmgmtNs
				pr.Name = "kubernetes-keppel.alerts"
				err := k8sClient.Delete(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				err = k8sClient.Get(ctx, newObjKey(resmgmtNs, k8sAbsentPRName), &pr)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})

			It("should delete orphaned absent alert rules from "+osAbsentPRName+" in "+resmgmtNs+" namespace", func() {
				// AbsentPromRule 'openstack-absent-metric-alert-rules' in 'resmgmt'
				// namespace holds the aggregate absent alert rules for both
				// 'openstack-limes-api.alerts' and 'openstack-limes-roleassign.alerts'
				// PromRules.
				limesRolePRName := "openstack-limes-roleassign.alerts"
				var pr monitoringv1.PrometheusRule
				pr.Namespace = resmgmtNs
				pr.Name = limesRolePRName
				err := k8sClient.Delete(ctx, &pr)
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
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, newObjKey(resmgmtNs, osAbsentPRName), &actual)
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
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, newObjKey(resmgmtNs, "openstack-limes-api.alerts"), &pr)
				Expect(err).ToNot(HaveOccurred())
				pr.Spec.Groups = []monitoringv1.RuleGroup{}
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				err = k8sClient.Get(ctx, newObjKey("resmgmt", osAbsentPRName), &pr)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})
		})
	})

	Describe("Disabling", func() {
		objKey := newObjKey(swiftNs, "openstack-swift.alerts")
		prObjKey := newObjKey(swiftNs, osAbsentPRName)

		Context("for a specific alert rule", func() {
			It("should delete the corresponding absent alert rule from "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Get the corresponding AbsentPromRule and remove an absent alert rule
				// manually so that we can use for test assertion. We use first rule of
				// first group for this.
				var expected monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, prObjKey, &expected)
				Expect(err).ToNot(HaveOccurred())
				expected.Spec.Groups[0].Rules = expected.Spec.Groups[0].Rules[1:]

				// Add the 'no_alert_on_absence' label to the first rule of first group in
				// the PromRule. This should result in the deletion of the corresponding
				// absent alert rule from the AbsentPromRule.
				var pr monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				pr.Spec.Groups[0].Rules[0].Labels["no_alert_on_absence"] = "true"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Check that the corresponding absent alert rule was removed.
				waitForControllerToProcess()
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})

		Context("for an entire PrometheusRule", func() {
			It("should delete "+osAbsentPRName+" in "+swiftNs+" namespace", func() {
				// Add the 'absent-metrics-operator/disable' label to the PromRule. This
				// should result in the deletion of the corresponding AbsentPromRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				pr.Labels["absent-metrics-operator/disable"] = "true"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				c.EnqueueAllPrometheusRules() // Force reconciliation
				waitForControllerToProcess()
				err = k8sClient.Get(ctx, prObjKey, &pr)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
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
	return monitoringv1.Rule{
		Alert: strings.Title(metric),
		Expr:  intstr.FromString(fmt.Sprintf("%s > 0", metric)),
		For:   "5m",
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
