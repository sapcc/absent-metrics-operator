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
	"fmt"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringinformers "github.com/coreos/prometheus-operator/pkg/client/informers/externalversions"
	monitoringlisters "github.com/coreos/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringclient "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"k8s.io/apimachinery/pkg/api/errors"
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
		utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
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

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the PrometheusRule
// resource with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the resource with this namespace/name
	promRule, err := c.promRuleLister.PrometheusRules(namespace).Get(name)
	if err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("PrometheusRule '%s' no longer exists in work queue", key))
			return nil
		}
		return err
	}

	// TODO: remove
	if promRule != nil {
		l := promRule.GetLabels()
		if v, ok := l["app"]; ok {
			level.Info(c.logger).Log("msg", "found PrometheusRule", "app", v)
		}
	}

	return nil
}
