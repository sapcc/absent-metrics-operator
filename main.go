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

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // load auth plugin
	"k8s.io/klog/v2"

	"github.com/sapcc/absent-metrics-operator/internal/controller"
	"github.com/sapcc/absent-metrics-operator/internal/version"
)

func main() {
	var logLevel, logFormat, kubeconfig, resyncPeriod string
	var threadiness int
	flagset := flag.CommandLine
	klog.InitFlags(flagset)
	flagset.StringVar(&logLevel, "log-level", logLevelInfo,
		fmt.Sprintf("Log level to use. Possible values: %s", strings.Join(availableLogLevels, ", ")))
	flagset.StringVar(&logFormat, "log-format", logFormatLogfmt,
		fmt.Sprintf("Log format to use. Possible values: %s", strings.Join(availableLogFormats, ", ")))
	flagset.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster")
	flagset.IntVar(&threadiness, "threadiness", 1, "The controller's threadiness (number of workers)")
	flagset.StringVar(&resyncPeriod, "resync-period", "30s", "The controller's resync period. Valid time units are 's', 'm', 'h'. Minimum acceptable value is 15s.")
	flagset.Parse(os.Args[1:])

	dur, err := time.ParseDuration(resyncPeriod)
	if err != nil {
		logFatalAndExit(fmt.Sprintf("could not parse resync period: %s", err.Error()))
	}
	if dur < 15*time.Second {
		logFatalAndExit(fmt.Sprintf("minimum acceptable value for resync period is 15s, got: %s", dur))
	}

	logger := getLogger(logFormat, logLevel)

	logger.Log("msg", "starting absent-metrics-operator",
		"version", version.Version, "git-commit", version.GitCommitHash, "build-date", version.BuildDate)

	c, err := controller.New(kubeconfig, dur, log.With(logger, "component", "controller"))
	if err != nil {
		logger.Log("msg", "could not instantiate controller", "err", err)
		os.Exit(1)
	}

	// Set up signal handling for graceful shutdown
	wg, ctx := setupSignalHandlerAndRoutineGroup(logger)

	wg.Go(func() error { return c.Run(threadiness, ctx.Done()) })

	if err := wg.Wait(); err != nil {
		logger.Log("msg", "unhandled error received", "err", err)
		os.Exit(1)
	}
}
