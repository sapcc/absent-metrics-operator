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

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/go-kit/kit/log/level"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
)

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
