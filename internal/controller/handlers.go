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

import "github.com/go-kit/kit/log/level"

func (c *Controller) handleRuleAdd(obj interface{}) {
	level.Info(c.logger).Log("msg", "handleRuleAdd called")
}

func (c *Controller) handleRuleUpdate(old, new interface{}) {
	level.Info(c.logger).Log("msg", "handleRuleUpdate called")
}

func (c *Controller) handleRuleDelete(obj interface{}) {
	level.Info(c.logger).Log("msg", "handleRuleDelete called")
}
