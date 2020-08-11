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

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // load auth plugin
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sapcc/absent-metrics-operator/internal/controller"
	"github.com/sapcc/absent-metrics-operator/internal/log"
	"github.com/sapcc/absent-metrics-operator/internal/signals"
)

// This info identifies a specific build of the app.
// version and gitCommitHash are set at compile time.
var (
	version       = "dev"
	gitCommitHash = "unknown"
	buildDate     = time.Now().UTC().Format(time.RFC3339)
)

var (
	availableLogLevels = []string{
		log.LevelAll,
		log.LevelDebug,
		log.LevelInfo,
		log.LevelWarn,
		log.LevelError,
		log.LevelNone,
	}
	availableLogFormats = []string{
		log.FormatLogfmt,
		log.FormatJSON,
	}
)

func main() {
	var logLevel, logFormat, kubeconfig, resyncPeriod string
	flagset := flag.CommandLine
	flagset.StringVar(&logLevel, "log-level", log.LevelInfo,
		fmt.Sprintf("Log level to use. Possible values: %s", strings.Join(availableLogLevels, ", ")))
	flagset.StringVar(&logFormat, "log-format", log.FormatLogfmt,
		fmt.Sprintf("Log format to use. Possible values: %s", strings.Join(availableLogFormats, ", ")))
	flagset.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster")
	flagset.StringVar(&resyncPeriod, "resync-period", "30s",
		"The controller's resync period. Valid time units are 's', 'm', 'h'. Minimum acceptable value is 15s")
	if err := flagset.Parse(os.Args[1:]); err != nil {
		logFatalf("could not parse flagset: %s", err.Error())
	}
	dur, err := time.ParseDuration(resyncPeriod)
	if err != nil {
		logFatalf("could not parse resync period: %s", err.Error())
	}
	if dur < 15*time.Second {
		logFatalf("minimum acceptable value for resync period is 15s, got: %s", dur)
	}

	logger, err := log.New(os.Stdout, logFormat, logLevel)
	if err != nil {
		logFatalf(err.Error())
	}

	logger.Info("msg", "starting absent-metrics-operator",
		"version", version, "git-commit", gitCommitHash, "build-date", buildDate)

	// Create controller
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logger.Fatal("msg", "instantiating cluster config failed", "err", err)
	}
	c, err := controller.New(cfg, dur, log.With(*logger, "component", "controller"))
	if err != nil {
		logger.Fatal("msg", "could not instantiate controller", "err", err)
	}

	// Set up signal handling for graceful shutdown
	wg, ctx := signals.SetupSignalHandlerAndRoutineGroup(logger)

	// Start controller
	wg.Go(func() error { return c.Run(ctx.Done()) })

	if err := wg.Wait(); err != nil {
		logger.Fatal("msg", "unhandled error received", "err", err)
	}
}

// logFatalf is used when there is no log.Logger.
func logFatalf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", a...)
	os.Exit(1)
}
