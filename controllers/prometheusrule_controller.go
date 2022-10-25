// Copyright 2022 SAP SE
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
	"context"
	"strings"
	"time"

	"github.com/go-logr/logr"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// requeueInterval is the interval after which each resource will be requeued.
//
// The controller manager does a periodic sync (10 hours by default) that reconciles all
// watched resources.
//
// We use requeueInterval to do this additional reconciliation as a liveness check to see
// if the operator is working as intended, and to insure against missed watch events.
var requeueInterval = 5 * time.Minute

// PrometheusRuleReconciler reconciles a PrometheusRule object.
type PrometheusRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger

	// KeepLabel is a map of labels that will be retained from the original alert rule and
	// passed on to its corresponding absent alert rule.
	KeepLabel KeepLabel
}

//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules/status,verbs=get;update;patch

// Reconcile is part of the main Kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *PrometheusRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("name", req.Name, "namespace", req.Namespace)

	// Get the current PrometheusRule from the API server.
	var promRule monitoringv1.PrometheusRule
	err := r.Get(ctx, req.NamespacedName, &promRule)
	switch {
	case err == nil:
		err = r.reconcileObject(ctx, req.NamespacedName, &promRule)
	case apierrors.IsNotFound(err):
		err = r.handleObjectNotFound(ctx, req.NamespacedName)
	default:
		// Handle err down below.
	}
	if err != nil {
		if e, ok := err.(*ruleGroupParseError); ok {
			// We choose to absorb the error here as returning the error would requeue the
			// resource for immediate processing and we'll be stuck parsing broken alert
			// rules. Instead, we wait for the next time the resource is updated or until
			// the requeueInterval is elapsed (whichever happens first).
			log.Error(e, "could not parse rule groups")
			return ctrl.Result{RequeueAfter: requeueInterval}, nil
		}
		// Requeue for later processing.
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrometheusRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

// handleObjectNotFound is a helper function for Reconcile(). It exists separately so that
// we can exit on error without making the `switch` in Reconcile() complex.
func (r *PrometheusRuleReconciler) handleObjectNotFound(ctx context.Context, key types.NamespacedName) error {
	log := r.Log.WithValues("name", key.Name, "namespace", key.Namespace)

	// Step 1: check if the object is a PrometheusRule or an AbsencePrometheusRule.
	if strings.HasSuffix(key.Name, absencePromRuleNameSuffix) {
		// In case that the AbsencePrometheusRule no longer exists we don't have to do any
		// further processing. If it still exists then it will be handled the next time it
		// is reconciled.
		return nil
	}

	// Step 2: if it's a PrometheusRule then perhaps this specific resource no longer
	// exists therefore we need to clean up any orphaned absence alert rules.
	log.Info("PrometheusRule no longer exists")
	err := r.cleanUpOrphanedAbsenceAlertRules(ctx, key, "")
	if err == nil {
		deleteReconcileGauge(key.Namespace, key.Name)
		log.Info("successfully cleaned up orphaned absence alert rules")
	}
	return err
}

// reconcileObject is a helper function for Reconcile(). It exists separately so that we
// can exit on error without making the `switch` in Reconcile() complex.
func (r *PrometheusRuleReconciler) reconcileObject(
	ctx context.Context,
	key types.NamespacedName,
	obj *monitoringv1.PrometheusRule,
) error {

	log := r.Log.WithValues("name", key.Name, "namespace", key.Namespace)
	l := obj.GetLabels()

	// Step 1: check if the object is a PrometheusRule or an AbsencePrometheusRule.
	if parseBool(l[labelOperatorManagedBy]) {
		// If it's an AbsencePrometheusRule then do a clean up, i.e. remove any absence
		// metric alert rules from it that no longer belong to any PrometheusRule.
		updatedAt, err := time.Parse(time.RFC3339, obj.Annotations[annotationOperatorUpdatedAt])
		if err != nil && time.Now().UTC().Sub(updatedAt) < requeueInterval {
			// No need for clean up if the AbsencePrometheusRule was updated recently.
			// We'll process it when it's next requeued.
			return nil
		}
		err = r.cleanUpAbsencePrometheusRule(ctx, obj)
		if err == nil {
			log.Info("successfully cleaned up AbsencePrometheusRule")
		}
		return err
	}

	// Step 2: if it's a PrometheusRule then check if the the operator has been disabled
	// for it.
	if parseBool(l[labelOperatorDisable]) {
		log.Info("operator disabled for this PrometheusRule")
		err := r.cleanUpOrphanedAbsenceAlertRules(ctx, key, l[labelPrometheusServer])
		if err == nil {
			deleteReconcileGauge(key.Namespace, key.Name)
			log.Info("successfully cleaned up orphaned absence alert rules")
		}
		return err
	}

	// Step 3: Generate the corresponding absence alert rules for this resource.
	err := r.updateAbsenceAlertRules(ctx, obj)
	if err == nil {
		setReconcileGauge(key.Namespace, key.Name, time.Now())
		log.Info("successfully reconciled PrometheusRule")
	}
	return err
}
