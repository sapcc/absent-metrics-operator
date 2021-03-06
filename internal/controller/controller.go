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

	"github.com/pkg/errors"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	informers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringlisters "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
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
	labelOperatorManagedBy = "absent-metrics-operator/managed-by"
	labelOperatorDisable   = "absent-metrics-operator/disable"

	labelNoAlertOnAbsence = "no_alert_on_absence"
)

// Common constants for reusability.
const (
	LabelTier    = "tier"
	LabelService = "service"
)

const (
	// DefaultResyncPeriod is the period after which the shared informer will
	// refresh its cache.
	DefaultResyncPeriod = 10 * time.Minute

	// reconciliationPeriod is the period after which all the objects in the
	// informer's cache and orphaned objects with absent alerts are added to
	// the workqueue so that they can be processed by the syncHandler().
	//
	// The informer calls the event handlers only if the resource state
	// changes. We do this additional reconciliation as a liveness check to see
	// if the operator is working as intended, and as manual maintenance for
	// cases when the informer has missed a PrometheusRule deletion/rename.
	reconciliationPeriod = 5 * time.Minute
)

// Controller is the controller implementation for acting on PrometheusRule
// resources.
type Controller struct {
	isTest  bool
	logger  *log.Logger
	metrics *Metrics

	keepLabel map[string]bool
	// keepTierServiceLabels is a shorthand for:
	//   c.keepLabel[LabelTier] && c.keepLabel[LabelService]
	keepTierServiceLabels bool

	kubeClientset kubernetes.Interface
	promClientset monitoringclient.Interface

	promRuleInformer cache.SharedIndexInformer
	promRuleLister   monitoringlisters.PrometheusRuleLister
	workqueue        workqueue.RateLimitingInterface
}

// Opts holds the data required for instantiating a Controller.
type Opts struct {
	IsTest             bool
	Logger             *log.Logger
	KeepLabel          map[string]bool
	PrometheusRegistry prometheus.Registerer
	Config             *rest.Config
	ResyncPeriod       time.Duration
}

// New creates a new Controller.
func New(opts Opts) (*Controller, error) {
	kubeClient, err := kubernetes.NewForConfig(opts.Config)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating kubernetes client failed")
	}

	pClient, err := monitoringclient.NewForConfig(opts.Config)
	if err != nil {
		return nil, errors.Wrap(err, "instantiating monitoring client failed")
	}

	c := &Controller{
		isTest:        opts.IsTest,
		logger:        opts.Logger,
		metrics:       NewMetrics(opts.PrometheusRegistry),
		keepLabel:     opts.KeepLabel,
		kubeClientset: kubeClient,
		promClientset: pClient,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "prometheusrules"),
	}
	c.keepTierServiceLabels = c.keepLabel[LabelTier] && c.keepLabel[LabelService]
	ruleInf := informers.NewSharedInformerFactory(pClient, opts.ResyncPeriod).Monitoring().V1().PrometheusRules()
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
	// the operator itself.
	l := obj.(*monitoringv1.PrometheusRule).GetLabels()
	if mustParseBool(l[labelOperatorManagedBy]) {
		return
	}

	c.workqueue.Add(key)
}

// EnqueueAllPrometheusRules adds all PrometheusRules to the workqueue.
// This method is exported because it is used in the unit tests to force reconciliation.
func (c *Controller) EnqueueAllPrometheusRules() {
	// promRules is a map of namespace to map[promRuleName]bool.
	promRules := make(map[string]map[string]bool)
	// absentPromRules is a map of namespace to a slice of absentPrometheusRule.
	absentPromRules := make(map[string][]*monitoringv1.PrometheusRule)

	// Add all PrometheusRules in the shared informer's cache to the workqueue
	// and get names of all PrometheusRules that exist in the informer's cache.
	objs := c.promRuleInformer.GetStore().List()
	for _, v := range objs {
		pr, ok := v.(*monitoringv1.PrometheusRule)
		if !ok {
			continue
		}

		ns := pr.GetNamespace()
		n := pr.GetName()

		if l := pr.GetLabels(); mustParseBool(l[labelOperatorManagedBy]) {
			absentPromRules[ns] = append(absentPromRules[ns], pr)
		} else {
			if _, ok = promRules[ns]; !ok {
				promRules[ns] = make(map[string]bool)
			}
			promRules[ns][n] = true
			c.enqueuePromRule(pr)
		}
	}

	// Enqueue the PrometheusRules which no longer exist but their corresponding
	// absent alerts still exist in an absentPrometheusRule.
	// The syncHandler() will perform the appropriate cleanup for them.
	for ns, promRuleNames := range promRules {
		// Check all absentPrometheusRules for this namespace for alerts that
		// don't belong to any PrometheusRule in promRuleNames.
		for _, aPR := range absentPromRules[ns] {
			// A map is used because there could be multiple RuleGroups that
			// contain absent alerts concerning a single PrometheusRule therefore
			// we check all the groups before adding it to the queue.
			shouldAdd := make(map[string]struct{})
			for _, g := range aPR.Spec.Groups {
				n := getPromRulefromAbsentRuleGroup(g.Name)
				if n != "" && !promRuleNames[n] {
					shouldAdd[n] = struct{}{}
				}
			}

			for n := range shouldAdd {
				key := fmt.Sprintf("%s/%s", ns, n)
				c.workqueue.Add(key)
			}
		}
	}
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	done := make(chan struct{})
	reconcileT := time.NewTicker(reconciliationPeriod)
	defer reconcileT.Stop()
	go func() {
		for {
			select {
			case <-done:
				return
			case <-reconcileT.C:
				c.EnqueueAllPrometheusRules()
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
	c.logger.Debug("msg", "sync successful", "key", key)
	return true
}

// syncHandler gets a PrometheusRule from the queue and updates the
// corresponding absentPrometheusRule for it.
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
		if l := promRule.GetLabels(); mustParseBool(l[labelOperatorDisable]) {
			c.logger.Debug("msg", "operator disabled for this PrometheusRule, cleaning up any corresponding absent alerts", "key", key)
			err = c.cleanUpOrphanedAbsentAlertsNamespace(name, namespace)
			if err == nil {
				c.metrics.SuccessfulPrometheusRuleReconcileTime.DeleteLabelValues(namespace, name)
				return nil
			}
		} else {
			err = c.updateAbsentAlerts(namespace, name, promRule)
		}
	case apierrors.IsNotFound(err):
		// The resource may no longer exist, in which case we clean up any
		// orphaned absent alerts.
		c.logger.Debug("msg", "PrometheusRule no longer exists", "key", key)
		err = c.cleanUpOrphanedAbsentAlertsNamespace(name, namespace)
		if err == nil {
			c.metrics.SuccessfulPrometheusRuleReconcileTime.DeleteLabelValues(namespace, name)
			return nil
		}
	default:
		// Requeue object for later processing.
		return err
	}
	if err != nil {
		if e, ok := err.(*errParseRuleGroups); ok {
			// We choose to absorb the error here as the worker would requeue
			// the resource otherwise and we'll be stuck parsing broken alert
			// rules. Instead we wait for the next time the resource is updated
			// and requeued.
			c.logger.ErrorWithBackoff("msg", "could not parse rule groups", "key", key, "err", e)
			return nil
		}
		return err
	}

	gauge := c.metrics.SuccessfulPrometheusRuleReconcileTime.WithLabelValues(namespace, name)
	if c.isTest {
		gauge.Set(float64(1))
	} else {
		gauge.SetToCurrentTime()
	}

	return nil
}

type errParseRuleGroups struct {
	cause error
}

// Error implements the error interface.
func (e *errParseRuleGroups) Error() string {
	return e.cause.Error()
}

func (c *Controller) updateAbsentAlerts(namespace, name string, promRule *monitoringv1.PrometheusRule) error {
	promRuleLabels := promRule.GetLabels()
	// Find the Prometheus server for this resource.
	promServer, ok := promRuleLabels["prometheus"]
	if !ok {
		// This shouldn't happen but just in case it does.
		return errors.New("no 'prometheus' label found")
	}

	// Get the corresponding absentPrometheusRule.
	existingAbsentPromRule := false
	absentPromRule, err := c.getExistingAbsentPrometheusRule(namespace, promServer)
	switch {
	case err == nil:
		existingAbsentPromRule = true
	case apierrors.IsNotFound(err):
		absentPromRule, err = c.newAbsentPrometheusRule(namespace, promServer)
		if err != nil {
			return errors.Wrap(err, "could not initialize new AbsentPrometheusRule")
		}
	default:
		// This could have been caused by a temporary network failure, or any
		// other transient reason.
		return errors.Wrap(err, "could not get existing AbsentPrometheusRule")
	}

	defaultTier := absentPromRule.Tier
	defaultService := absentPromRule.Service
	if c.keepTierServiceLabels {
		// If the PrometheusRule has tier and service labels then use those as
		// the defaults.
		if t := promRuleLabels[LabelTier]; t != "" {
			defaultTier = t
		}
		if s := promRuleLabels[LabelService]; s != "" {
			defaultService = s
		}
	}
	// Parse alert rules into absent alert rules.
	absentRg, err := c.parseRuleGroups(name, defaultTier, defaultService, promRule.Spec.Groups)
	if err != nil {
		return &errParseRuleGroups{cause: err}
	}

	if len(absentRg) == 0 {
		if existingAbsentPromRule {
			// This can happen when changes have been made to alert rules that
			// result in no absent alerts.
			// E.g. absent() or the 'no_alert_on_absence' label was used.
			// In this case we clean up orphaned absent alerts.
			return c.cleanUpOrphanedAbsentAlerts(name, absentPromRule)
		}
		return nil
	}

	if c.keepTierServiceLabels {
		key := fmt.Sprintf("%s/%s", namespace, name)
		if defaultTier == "" {
			c.logger.Info("msg", "could not find a value for 'tier' label", "key", key)
		}
		if defaultService == "" {
			c.logger.Info("msg", "could not find a value for 'service' label", "key", key)
		}
	}
	if existingAbsentPromRule {
		err = c.updateAbsentPrometheusRule(absentPromRule, absentRg)
	} else {
		absentPromRule.Spec.Groups = absentRg
		_, err = c.promClientset.MonitoringV1().PrometheusRules(namespace).
			Create(context.Background(), absentPromRule.PrometheusRule, metav1.CreateOptions{})
	}
	return err
}
