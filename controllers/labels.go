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

// These constants are exported for reusability across packages.
const (
	LabelCCloudSupportGroup = "ccloud/support-group"
	LabelCCloudService      = "ccloud/service"

	LabelSupportGroup = "support_group"
	LabelTier         = "tier"
	LabelService      = "service"
)

const (
	annotationOperatorUpdatedAt = "absent-metrics-operator/updated-at"

	labelOperatorManagedBy = "absent-metrics-operator/managed-by"
	labelOperatorDisable   = "absent-metrics-operator/disable"

	labelNoAlertOnAbsence = "no_alert_on_absence"
	labelPrometheusServer = "prometheus"
	labelGreenhousePlugin = "plugin"
	labelThanosRuler      = "thanos-ruler"
)

// KeepLabel specifies which labels to keep on an absence alert rule.
type KeepLabel map[string]bool
