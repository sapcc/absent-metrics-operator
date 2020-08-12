// Parts of this file have been borrowed from github.com/kubernetes/sample-controller
// which is released under Apache-2.0 License with notice:
// Copyright 2017 The Kubernetes Authors
//
// The rest of the source code is licensed under:
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
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	informers "github.com/coreos/prometheus-operator/pkg/client/informers/externalversions"
	monitoringlisters "github.com/coreos/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringclient "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/sapcc/absent-metrics-operator/internal/log"
)

const (
	labelManagedBy = "absent-metrics-operator/managed-by"
	labelDisable   = "absent-metrics-operator/disable"
)

const (
	// DefaultResyncPeriod is the period after which the shared informer will
	// refresh its cache.
	DefaultResyncPeriod = 10 * time.Minute

	// DefaultReconciliationPeriod is the period after which all the objects in
	// the informer's cache will be added to the workqueue so that they can be
	// processed by the syncHandler().
	//
	// The informer calls the event handlers only if the resource state
	// changes. We force this additional reconciliation as a liveness check to
	// see if the operator is working as intended.
	DefaultReconciliationPeriod = 5 * time.Minute
)

// Controller is the controller implementation for acting on PrometheusRule
// resources.
type Controller struct {
	logger  *log.Logger
	metrics *Metrics

	kubeClientset kubernetes.Interface
	promClientset monitoringclient.Interface

	promRuleInformer cache.SharedIndexInformer
	promRuleLister   monitoringlisters.PrometheusRuleLister
	workqueue        workqueue.RateLimitingInterface
}

// New creates a new Controller.
func New(cfg *rest.Config, resyncPeriod time.Duration, r prometheus.Registerer, logger *log.Logger) (*Controller, error) {
	kClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client failed")
	}

	pClient, err := monitoringclient.NewForConfig(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating monitoring client failed")
	}

	c := &Controller{
		logger:        logger,
		metrics:       NewMetrics(r),
		kubeClientset: kClient,
		promClientset: pClient,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "prometheusrules"),
	}
	ruleInf := informers.NewSharedInformerFactory(pClient, resyncPeriod).Monitoring().V1().PrometheusRules()
	c.promRuleLister = ruleInf.Lister()
	c.promRuleInformer = ruleInf.Informer()
	c.promRuleInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuePromRule,
		UpdateFunc: func(old, new interface{}) {
			newRule := new.(*monitoringv1.PrometheusRule)
			oldRule := old.(*monitoringv1.PrometheusRule)
			if newRule.ResourceVersion == oldRule.ResourceVersion {
				// Periodic resync will send update events for all known
				// PrometheusRules. Two different versions of the same
				// PrometheusRule will always have different RVs.
				return
			}
			c.enqueuePromRule(new)
		},
		DeleteFunc: c.enqueuePromRule,
	})

	return c, nil
}

// Run will sync informer caches and start workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(stopCh <-chan struct{}) error {
	defer c.workqueue.ShutDown()

	c.logger.Info("msg", "starting controller")

	errChan := make(chan error)
	go func() {
		v, err := c.kubeClientset.Discovery().ServerVersion()
		if err == nil {
			c.logger.Info("msg", "connection established", "cluster-version", v)
		}
		errChan <- err
	}()
	select {
	case err := <-errChan:
		if err != nil {
			return errors.Wrap(err, "communication with server failed")
		}
	case <-stopCh:
		return nil
	}

	go c.promRuleInformer.Run(stopCh)

	c.logger.Info("msg", "waiting for informer cache to sync")
	if !cache.WaitForCacheSync(stopCh, c.promRuleInformer.HasSynced) {
		return errors.New("failed to sync informer cache")
	}

	c.logger.Info("msg", "starting worker")
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
	c.logger.Info("msg", "shutting down workers")

	return nil
}

// enqueuePromRule takes a PrometheusRule resource and converts it into a
// namespace/name string which is then put onto the workqueue. This method
// should *not* be passed resources of any type other than PrometheusRule.
func (c *Controller) enqueuePromRule(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.logger.ErrorWithBackoff("msg", "could not create key for object", "err", err)
		return
	}

	// Do not enqueue object to workqueue if it is managed (read: created) by
	// the operator itself or if the annotation for disabling the operator is
	// present.
	l := obj.(*monitoringv1.PrometheusRule).GetLabels()
	if l[labelManagedBy] == "true" {
		return
	}
	if l[labelDisable] == "true" {
		c.logger.Info("msg", "operator disabled, skipping", "key", key)
		return
	}

	c.workqueue.Add(key)
}

// enqueueAllObjects adds all objects in the shared informer's cache to the
// workqueue.
func (c *Controller) enqueueAllObjects() {
	objs := c.promRuleInformer.GetStore().List()
	for _, v := range objs {
		if pr, ok := v.(*monitoringv1.PrometheusRule); ok {
			c.enqueuePromRule(pr)
		}
	}
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	ticker := time.NewTicker(DefaultReconciliationPeriod)
	defer ticker.Stop()
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				c.enqueueAllObjects()
			}
		}
	}()

	for c.processNextWorkItem() {
	}

	done <- struct{}{}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()
	if shutdown {
		return false
	}

	// We defer Done here so the workqueue knows we have finished
	// processing this item.
	defer c.workqueue.Done(obj)

	// We expect strings to come off the workqueue. These are of the
	// form namespace/name.
	key, ok := obj.(string)
	if !ok {
		// As the item in the workqueue is actually invalid, we call
		// Forget here else we'd go into a loop of attempting to
		// process an invalid work item.
		c.workqueue.Forget(obj)
		c.logger.ErrorWithBackoff("msg", fmt.Sprintf("expected string in work queue but got %#v", obj))
		return true
	}

	if err := c.syncHandler(key); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.workqueue.AddRateLimited(obj)
		c.logger.ErrorWithBackoff("msg", "could not sync object, requeuing", "key", key, "err", err)
		return true
	}

	// Finally, if no errors occurred we Forget this item so it does not
	// get queued again until another change happens.
	c.workqueue.Forget(obj)
	c.logger.Info("msg", "sync successful", "key", key)
	return true
}

// syncHandler gets a PrometheusRule from the queue and updates the
// corresponding absent metric alert PrometheusRule for it.
//nolint:gocyclo,gocognit
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.logger.ErrorWithBackoff("msg", "invalid resource key", "key", key)
		return nil
	}

	// Get the PrometheusRule with this namespace/name.
	promRule, err := c.promRuleLister.PrometheusRules(namespace).Get(name)
	switch {
	case err == nil:
		// continue processing down below
	case apierrors.IsNotFound(err):
		// The resource may no longer exist, in which case we clean up any
		// orphaned absent alert rules.
		c.logger.Info("msg", "PrometheusRule no longer exists", "key", key)
		return c.deleteAbsentAlertRulesNamespace(namespace, name)
	default:
		// Requeue object for later processing.
		return err
	}

	// Find the Prometheus server for this resource.
	promServerName, ok := promRule.Labels["prometheus"]
	if !ok {
		// This shouldn't happen but just in case it does.
		c.logger.ErrorWithBackoff("msg", "no 'prometheus' label found on the PrometheusRule", "key", key)
		return nil
	}

	// Default tier and service label values to use for absent metric alerts.
	// See parseRuleGroups() for info on why we need this.
	var tier, service string

	// Get the PrometheusRule resource that defines the absent metric alert
	// rules for this namespace.
	ctx := context.Background()
	absentPromRuleExists := false
	absentPromRuleName := fmt.Sprintf("%s-absent-metric-alert-rules", promServerName)
	absentPromRule, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).Get(ctx, absentPromRuleName, metav1.GetOptions{})
	switch {
	case err == nil:
		absentPromRuleExists = true
		tier, service = getTierAndService(absentPromRule.Spec.Groups)
	case apierrors.IsNotFound(err):
		// Try to get a value for tier and service by traversing through all the
		// PrometheusRules for the specific Prometheus server in this namespace.
		prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			// Requeue object for later processing.
			return errors.Wrap(err, "could not list PrometheusRules")
		}
		var rg []monitoringv1.RuleGroup
		for _, pr := range prList.Items {
			if v := pr.Labels["prometheus"]; v == promServerName {
				rg = append(rg, pr.Spec.Groups...)
			}
		}
		tier, service = getTierAndService(rg)
	default:
		// This could have been caused by a temporary network failure, or any
		// other transient reason. Requeue object for later processing.
		return errors.Wrap(err, "could not get absent PrometheusRule "+absentPromRuleName)
	}
	if tier == "" || service == "" {
		// We shouldn't arrive at this point because this would mean that there
		// was not a single alert rule for the prometheus server in this
		// namespace that did not use templating for its tier and service
		// labels.
		// Requeue object for later processing.
		return errors.New("could not find default tier and service")
	}

	// Parse alert rules into absent metric alert rules.
	rg, err := parseRuleGroups(name, tier, service, promRule.Spec.Groups)
	if err != nil {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise and we'll be stuck parsing broken alert rules.
		// Instead we wait for the next time the resource is updated and
		// requeued.
		c.logger.ErrorWithBackoff("msg", "could not parse rule groups", "key", key, "err", err)
		return nil
	}

	switch lenRg := len(rg); {
	case lenRg == 0 && absentPromRuleExists:
		// This can happen when changes have been made to a PrometheusRule that
		// result in no absent alert rules. E.g. absent() operator was used. In
		// this case we clean up orphaned absent alert rules.
		err = c.deleteAbsentAlertRules(namespace, name, absentPromRule)
	case lenRg > 0 && absentPromRuleExists:
		err = c.updateAbsentPrometheusRule(namespace, absentPromRule, rg)
	case lenRg > 0:
		err = c.createAbsentPrometheusRule(namespace, absentPromRuleName, promServerName, rg)
	}
	if err != nil {
		return err
	}

	c.metrics.SuccessfulPrometheusRuleReconcileTime.WithLabelValues(namespace, name).SetToCurrentTime()
	return nil
}
