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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sapcc/absent-metrics-operator/test/fixtures"
)

// Wait for controller to resync and complete its processing.
var waitForControllerToProcess = func() { time.Sleep(500 * time.Millisecond) }

var _ = Describe("Controller", func() {
	ctx := context.Background()

	Describe("AbsentPrometheusRule creation", func() {
		It("should create "+fixtures.K8sAbsentPromRuleName+" in resmgmt namespace", func() {
			expected := fixtures.ResMgmtK8sAbsentPromRule
			var actual monitoringv1.PrometheusRule
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: fixtures.K8sAbsentPromRuleName}, &actual)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Labels).To(Equal(expected.Labels))
			Expect(actual.Spec).To(Equal(expected.Spec))
		})

		It("should create "+fixtures.OSAbsentPromRuleName+" in resmgmt namespace", func() {
			expected := fixtures.ResMgmtOSAbsentPromRule
			var actual monitoringv1.PrometheusRule
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: fixtures.OSAbsentPromRuleName}, &actual)
			Expect(err).ToNot(HaveOccurred())
			Expect(actual.Labels).To(Equal(expected.Labels))
			Expect(actual.Spec).To(Equal(expected.Spec))
		})

		Context("with disabled label", func() {
			prObjKey := client.ObjectKey{Namespace: "swift", Name: fixtures.OSAbsentPromRuleName}

			It("should not create "+fixtures.OSAbsentPromRuleName+" in swift namespace", func() {
				var actual monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})

			It("should create "+fixtures.OSAbsentPromRuleName+" in swift namespace after removing disable label", func() {
				// Get the concerning PrometheusRule resource.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "swift", Name: "openstack-swift.alerts"}, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Update the "disable" label on the PrometheusRule.
				pr.Labels["absent-metrics-operator/disable"] = "false"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Controller should have created the corresponding Absent
				// PrometheusRule.
				waitForControllerToProcess()
				expected := fixtures.SwiftOSAbsentPromRule
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Labels).To(Equal(expected.Labels))
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})
	})

	Describe("AbsentPrometheusRule update", func() {
		objKey := client.ObjectKey{Namespace: "swift", Name: "openstack-swift.alerts"}
		prObjKey := client.ObjectKey{Namespace: "swift", Name: fixtures.OSAbsentPromRuleName}
		tier, service := "os", "swift"
		getMockRule := func(m string) monitoringv1.Rule {
			return monitoringv1.Rule{
				Alert: strings.Title(m),
				Expr:  intstr.FromString(fmt.Sprintf("%s > 0", m)),
				For:   "5m",
				Labels: map[string]string{
					"tier":    "tier",
					"service": "service",
				},
			}
		}

		Context("with adding a new alert rule", func() {
			It("should update "+fixtures.OSAbsentPromRuleName+" in swift namespace", func() {
				// Update the original PrometheusRule resource with a new alert rule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				rule := getMockRule("foo_bar")
				i := len(pr.Spec.Groups) - 1
				pr.Spec.Groups[i].Rules = append(pr.Spec.Groups[i].Rules, rule)
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Check if the corresponding AbsentPrometheusRule was updated.
				expected := fixtures.SwiftOSAbsentPromRule
				rL, err := c.ParseAlertRule(tier, service, rule) //nolint:typecheck
				Expect(err).ToNot(HaveOccurred())
				i = len(expected.Spec.Groups) - 1
				expected.Spec.Groups[i].Rules = append(expected.Spec.Groups[i].Rules, rL...)

				waitForControllerToProcess()
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})

		Context("with updating an existing alert rule", func() {
			It("should update "+fixtures.OSAbsentPromRuleName+" in swift namespace", func() {
				// Update an existing alert rule in the original PrometheusRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				rule := getMockRule("bar_foo")
				pr.Spec.Groups[0].Rules[0] = rule // first rule of first group
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				// Check if the corresponding AbsentPrometheusRule was updated.
				expected := fixtures.SwiftOSAbsentPromRule
				rL, err := c.ParseAlertRule(tier, service, rule)
				Expect(err).ToNot(HaveOccurred())
				expected.Spec.Groups[0].Rules[0] = rL[0]

				waitForControllerToProcess()
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})
	})

	Describe("AbsentPrometheusRule cleanup", func() {
		Context("with PrometheusRule deletion", func() {
			It("should delete the "+fixtures.K8sAbsentPromRuleName+" from resmgmt namespace", func() {
				var pr monitoringv1.PrometheusRule
				pr.Name = "kubernetes-keppel.alerts"
				pr.Namespace = "resmgmt"
				err := k8sClient.Delete(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: fixtures.K8sAbsentPromRuleName}, &pr)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})

			It("should delete orphaned absent metric alert rule from "+fixtures.OSAbsentPromRuleName+" in resmgmt namespace", func() {
				// Since "openstack-absent-metric-alert-rules" in "resmgmt"
				// namespace holds the aggregate absent metric alert rules for
				// the "openstack-limes-api.alerts" and
				// "openstack-limes-roleassign.alerts", deleting one resource
				// should only result in cleanup of its corresponding alerts,
				// not the deletion of the entire AbsentPrometheusRule
				// resource.
				var pr monitoringv1.PrometheusRule
				pr.Name = "openstack-limes-roleassign.alerts"
				pr.Namespace = "resmgmt"
				err := k8sClient.Delete(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				expected := fixtures.ResMgmtOSAbsentPromRule
				expected.Spec.Groups = []monitoringv1.RuleGroup{fixtures.LimesAbsentAlertsAPIGroup}
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: fixtures.OSAbsentPromRuleName}, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})

		Context("with PrometheusRule update", func() {
			It("should delete the "+fixtures.OSAbsentPromRuleName+" from resmgmt namespace", func() {
				// We will remove alert rules from the original PrometheusRule
				// which should result in deletion of the corresponding Absent
				// PrometheusRule since there are no corresponding absent
				// metric alert rules.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: "openstack-limes-api.alerts"}, &pr)
				Expect(err).ToNot(HaveOccurred())
				pr.Spec.Groups = []monitoringv1.RuleGroup{}
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				err = k8sClient.Get(ctx, client.ObjectKey{Namespace: "resmgmt", Name: fixtures.OSAbsentPromRuleName}, &pr)
				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsNotFound(err)).To(Equal(true))
			})
		})
	})

	Describe("Operator disable", func() {
		objKey := client.ObjectKey{Namespace: "swift", Name: "openstack-swift.alerts"}
		prObjKey := client.ObjectKey{Namespace: "swift", Name: fixtures.OSAbsentPromRuleName}

		Context("with disabling operator for a specific alert rule", func() {
			It("should delete the corresponding absent alert rule from "+fixtures.OSAbsentPromRuleName+" in swift namespace", func() {
				// Add the no_alert_on_absence label to a specific alert rule.
				// This should result in the deletion of the corresponding
				// absent alert rule.
				var expected monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, prObjKey, &expected)
				Expect(err).ToNot(HaveOccurred())
				// Remove first rule of first group
				expected.Spec.Groups[0].Rules = expected.Spec.Groups[0].Rules[1:]

				var pr monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				// Add no_alert_on_absence label to first rule of first group.
				pr.Spec.Groups[0].Rules[0].Labels["no_alert_on_absence"] = "true"
				err = k8sClient.Update(ctx, &pr)
				Expect(err).ToNot(HaveOccurred())

				waitForControllerToProcess()
				var actual monitoringv1.PrometheusRule
				err = k8sClient.Get(ctx, prObjKey, &actual)
				Expect(err).ToNot(HaveOccurred())
				Expect(actual.Spec).To(Equal(expected.Spec))
			})
		})

		Context("with disabling operator for a PrometheusRule", func() {
			It("should delete the "+fixtures.OSAbsentPromRuleName+" in swift namespace", func() {
				// Add the disable label to the original PrometheusRule
				// resource. This should result in the deletion of the
				// corresponding AbsentPrometheusRule.
				var pr monitoringv1.PrometheusRule
				err := k8sClient.Get(ctx, objKey, &pr)
				Expect(err).ToNot(HaveOccurred())
				pr.Labels["absent-metrics-operator/disable"] = "true"
				// pr.Spec.Groups = []monitoringv1.RuleGroup{}
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

	Describe("AbsentPrometheusRule orphaned gauge metrics", func() {
		It("should be cleaned up", func() {
			handler := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
			method := http.MethodGet
			path := "/metrics"
			var requestBody io.Reader
			request := httptest.NewRequest(method, path, requestBody)

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			response := recorder.Result()
			Expect(response.StatusCode).To(Equal(200))
			responseBytes, err := ioutil.ReadAll(response.Body)
			Expect(err).ToNot(HaveOccurred())

			// Write actual content to file to make it easy to copy the
			// computed result over to the fixture path when a new test is
			// added or an existing one is modified.
			fixturePath, err := filepath.Abs("fixtures/metrics.prom")
			Expect(err).ToNot(HaveOccurred())
			actualPath := fixturePath + ".actual"
			err = ioutil.WriteFile(actualPath, responseBytes, 0600)
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
