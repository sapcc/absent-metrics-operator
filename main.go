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
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sapcc/go-bits/httpee"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // load auth plugin
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sapcc/absent-metrics-operator/internal/controller"
	"github.com/sapcc/absent-metrics-operator/internal/log"
	"github.com/sapcc/absent-metrics-operator/internal/signals"
)

// This info identifies a specific build of the app. It is set at compile time.
var (
	version = "dev"
	commit  = "unknown"
	date    = "now"
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
	defaultKeepLabels = []string{
		controller.LabelService,
		controller.LabelTier,
	}
)

func main() {
	var logLevel, logFormat, kubeconfig, keepLabels string
	flagset := flag.CommandLine
	flagset.StringVar(&logLevel, "log-level", log.LevelInfo,
		fmt.Sprintf("Log level to use. Possible values: %s", strings.Join(availableLogLevels, ", ")))
	flagset.StringVar(&logFormat, "log-format", log.FormatLogfmt,
		fmt.Sprintf("Log format to use. Possible values: %s", strings.Join(availableLogFormats, ", ")))
	flagset.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster")
	flagset.StringVar(&keepLabels, "keep-labels", strings.Join(defaultKeepLabels, ","),
		"A comma separated list of labels to keep from the original alert rule")
	if err := flagset.Parse(os.Args[1:]); err != nil {
		logFatalf("could not parse flagset: %s", err.Error())
	}

	logger, err := log.New(os.Stdout, logFormat, logLevel)
	if err != nil {
		logFatalf(err.Error())
	}

	logger.Info("msg", "starting absent-metrics-operator",
		"version", version, "git-commit", commit, "build-date", date)

	r := prometheus.NewRegistry()

	keepLabelMap := make(map[string]bool)
	kL := strings.Split(keepLabels, ",")
	for _, v := range kL {
		keepLabelMap[strings.TrimSpace(v)] = true
	}
	if keepLabelMap[controller.LabelTier] || keepLabelMap[controller.LabelService] {
		if !keepLabelMap[controller.LabelTier] && !keepLabelMap[controller.LabelService] {
			logger.Fatal("msg", "labels 'tier' and 'service' are co-dependent, i.e. use both or neither")
		}
	}

	// Create controller
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		logger.Fatal("msg", "instantiating cluster config failed", "err", err)
	}
	c, err := controller.New(cfg, controller.DefaultResyncPeriod, r, keepLabelMap, log.With(*logger, "component", "controller"))
	if err != nil {
		logger.Fatal("msg", "could not instantiate controller", "err", err)
	}

	// Set up signal handling for graceful shutdown
	wg, ctx := signals.SetupSignalHandlerAndRoutineGroup(logger)

	// Serve metrics at port "9659". This port has been allocated for absent
	// metrics operator.
	// See: https://github.com/prometheus/prometheus/wiki/Default-port-allocations
	listenAddr := ":9659"
	http.HandleFunc("/", landingPageHandler(logger))
	http.Handle("/metrics", promhttp.HandlerFor(r, promhttp.HandlerOpts{}))
	logger.Info("msg", "listening on "+listenAddr)
	wg.Go(func() error { return httpee.ListenAndServeContext(ctx, listenAddr, nil) })

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

func landingPageHandler(logger *log.Logger) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pageBytes := []byte(`<html>
		<head><title>Absent Metrics Operator</title></head>
		<body>
		<h1>Absent Metrics Operator</h1>
		<p><a href="/metrics">Metrics</a></p>
		<p><a href="https://github.com/sapcc/absent-metrics-operator">Source Code</a></p>
		</body>
		</html>`)

		if _, err := w.Write(pageBytes); err != nil {
			logger.ErrorWithBackoff("msg", "could not write landing page bytes", "err", err)
		}
	}
}
