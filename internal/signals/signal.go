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

package signals

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"golang.org/x/sync/errgroup"

	"github.com/sapcc/absent-metrics-operator/internal/log"
)

var onlyOneSignalHandler = make(chan struct{})

// SetupSignalHandlerAndRoutineGroup sets up a signal handler for for SIGTERM
// and SIGINT, and returns an errgroup.Group and a context which can be used to
// launch new goroutines.
func SetupSignalHandlerAndRoutineGroup(logger *log.Logger) (*errgroup.Group, context.Context) {
	close(onlyOneSignalHandler) // panics when called twice

	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	c := make(chan os.Signal, 1)
	signal.Notify(c, shutdownSignals...)
	go func() {
		select {
		case s := <-c:
			logger.Info("msg", fmt.Sprintf("received %s signal, exiting gracefully", s.String()))
		case <-ctx.Done():
		}
		cancel()
	}()

	return wg, ctx
}
