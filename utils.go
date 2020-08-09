// Copyright 2016 The prometheus-operator Authors
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
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"golang.org/x/sync/errgroup"

	"github.com/sapcc/absent-metrics-operator/internal/log"
)

var onlyOneSignalHandler = make(chan struct{})

func setupSignalHandlerAndRoutineGroup(logger log.Logger) (*errgroup.Group, context.Context) {
	close(onlyOneSignalHandler) // panics when called twice

	ctx, cancel := context.WithCancel(context.Background())
	wg, ctx := errgroup.WithContext(ctx)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		select {
		case s := <-c:
			logger.Log("msg", fmt.Sprintf("received %s signal, exiting gracefully...", s.String()))
		case <-ctx.Done():
		}
		cancel()
	}()

	return wg, ctx
}
