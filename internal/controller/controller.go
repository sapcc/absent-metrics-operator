// Copyright 2017 The Kubernetes Authors
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
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

const (
	labelManagedBy = "absent-metrics-operator/managed-by"
	labelDisable   = "absent-metrics-operator/disable"
)

// Controller is the controller implementation for acting on PrometheusRule
// resources.
type Controller struct {
	logger log.Logger

	kubeClientset kubernetes.Interface
	promClientset monitoringclient.Interface

	promRuleInformer cache.SharedIndexInformer
	promRuleLister   monitoringlisters.PrometheusRuleLister
	workqueue        workqueue.RateLimitingInterface
}

// New creates a new Controller.
func New(kubeconfig string, resyncPeriod time.Duration, logger log.Logger) (*Controller, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("instantiating cluster config failed: %s", err.Error())
	}

	kClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("instantiating kubernetes client failed: %s", err.Error())
	}

	pClient, err := monitoringclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("instantiating monitoring client failed: %s", err.Error())
	}

	c := &Controller{
		logger:        logger,
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

	level.Info(c.logger).Log("msg", "starting controller")

	errChan := make(chan error)
	go func() {
		v, err := c.kubeClientset.Discovery().ServerVersion()
		if err == nil {
			level.Info(c.logger).Log("msg", "connection established", "cluster-version", v)
		}
		errChan <- err
	}()
	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("communication with server failed: %s", err.Error())
		}
	case <-stopCh:
		return nil
	}

	go c.promRuleInformer.Run(stopCh)

	level.Info(c.logger).Log("msg", "waiting for informer cache to sync")
	if !cache.WaitForCacheSync(stopCh, c.promRuleInformer.HasSynced) {
		return fmt.Errorf("failed to sync informer cache")
	}

	level.Info(c.logger).Log("msg", "starting worker")
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
	level.Info(c.logger).Log("msg", "shutting down workers")

	return nil
}

// enqueuePromRule takes a PrometheusRule resource and converts it into a
// namespace/name string which is then put onto the workqueue. This method
// should *not* be passed resources of any type other than PrometheusRule.
func (c *Controller) enqueuePromRule(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not create key for object: %s", err.Error()))
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
		level.Info(c.logger).Log("msg", "operator disabled, skipping", "key", key)
		return
	}

	c.workqueue.Add(key)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
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
		utilruntime.HandleError(fmt.Errorf("expected string in work queue but got %#v", obj))
		return true
	}

	if err := c.syncHandler(key); err != nil {
		// Put the item back on the workqueue to handle any transient errors.
		c.workqueue.AddRateLimited(obj)
		utilruntime.HandleError(fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error()))
		return true
	}

	// Finally, if no errors occurred we Forget this item so it does not
	// get queued again until another change happens.
	c.workqueue.Forget(obj)
	level.Info(c.logger).Log("msg", "sync successful", "key", key)
	return true
}

// syncHandler gets a PrometheusRule from the queue and updates the
// corresponding absent metrics alert PrometheusRule for it.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the PrometheusRule with this namespace/name.
	promRule, err := c.promRuleLister.PrometheusRules(namespace).Get(name)
	switch {
	case err == nil:
		// continue processing down below
	case errors.IsNotFound(err):
		// The resource may no longer exist, in which case we clean up any
		// orphaned absent alert rules.
		level.Info(c.logger).Log("msg", "PrometheusRule no longer exists in work queue", "key", key)
		if err := c.deleteAbsentAlertRulesNamespace(namespace, name); err != nil {
			// Requeue object for later processing.
			return fmt.Errorf("could not clean up orphaned absent alert rules: %s", err.Error())
		}
		level.Info(c.logger).Log("msg", "successfully cleaned up orphaned absent alert rules", "key", key)
		return nil
	default:
		// Requeue object for later processing.
		return err
	}

	// Find the Prometheus server for this resource.
	promServerName, ok := promRule.Labels["prometheus"]
	if !ok {
		// This shouldn't happen but just in case it does.
		utilruntime.HandleError(fmt.Errorf("no 'prometheus' label found on the PrometheusRule %s", key))
		return nil
	}

	// Get the PrometheusRule resource that defines the absent metrics alert
	// rules for this namespace.
	absentPromRuleName := fmt.Sprintf("%s-absent-metrics-alert-rules", promServerName)
	absentPromRule, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
		Get(context.TODO(), absentPromRuleName, metav1.GetOptions{})

	// Default tier and service label values to use for absent metric alerts.
	// See parseRuleGroups() for info on why we need this.
	var tier, service string
	absentPromRuleExists := false
	switch {
	case err == nil:
		absentPromRuleExists = true
		tier, service = c.getTierAndService(absentPromRule.Spec.Groups)
	case errors.IsNotFound(err):
		// Try to get a value for tier and service by traversing through
		// all the PrometheusRules for this namespace.
		prList, err := c.promClientset.MonitoringV1().PrometheusRules(namespace).
			List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			// Requeue object for later processing.
			return fmt.Errorf("could not list PrometheusRules: %s", err.Error())
		}
		var rg []monitoringv1.RuleGroup
		for _, pr := range prList.Items {
			rg = append(rg, pr.Spec.Groups...)
		}
		tier, service = c.getTierAndService(rg)
	default:
		// This could have been caused by a temporary network failure, or any
		// other transient reason. Requeue object for later processing.
		return fmt.Errorf("could not get absent alert PrometheusRule '%s/%s': %s",
			namespace, absentPromRuleName, err.Error())
	}
	if tier == "" || service == "" {
		// We shouldn't arrive at this point because this would mean that
		// there was not a single alert rule in the namespace that did not
		// use templating for its tier and service labels.
		// Requeue object for later processing.
		return fmt.Errorf("could not find default tier and service for '%s'", namespace)
	}

	// Parse alert rules into absent metric alert rules.
	rg, err := parseRuleGroups(name, tier, service, promRule.Spec.Groups)
	if err != nil {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise and we'll be stuck parsing broken alert rules.
		// Instead we wait for the next time the resource is updated and requeued.
		utilruntime.HandleError(fmt.Errorf("could not parse rule groups for '%s': %s", key, err.Error()))
		return nil
	}
	if len(rg) == 0 && absentPromRuleExists {
		// This can happen when changes have been made to a PrometheusRule
		// that result in no absent alert rules. E.g. absent() operator was used.
		if err := c.deleteAbsentAlertRules(namespace, name, absentPromRule); err != nil {
			// Requeue object for later processing.
			return fmt.Errorf("could not clean up orphaned absent alert rules: %s", err.Error())
		}
		level.Info(c.logger).Log("msg", "successfully cleaned up orphaned absent alert rules", "key", key)
		return nil
	}

	if absentPromRuleExists {
		err = c.updateAbsentPrometheusRule(namespace, absentPromRule, rg)
	} else {
		err = c.createAbsentPrometheusRule(namespace, absentPromRuleName, promServerName, rg)
	}
	return err
}
