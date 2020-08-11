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

package log

import (
	"fmt"
	"io"
	"os"

	gokitlog "github.com/go-kit/kit/log"
	gokitlevel "github.com/go-kit/kit/log/level"
)

// Different types of log Level.
const (
	LevelAll   = "all"
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
	LevelNone  = "none"
)

// Different types of log Format.
const (
	FormatLogfmt = "logfmt"
	FormatJSON   = "json"
)

// Logger wraps a go-kit/kit/log.Logger. We use it to define custom methods.
// The Logger is safe for concurrent use by multiple goroutines.
type Logger struct {
	gokitlog.Logger
}

// New returns a new Logger.
func New(w io.Writer, format, lvl string) (*Logger, error) {
	sw := gokitlog.NewSyncWriter(w)
	l := gokitlog.NewLogfmtLogger(sw)
	if format == FormatJSON {
		l = gokitlog.NewJSONLogger(sw)
	}
	switch lvl {
	case LevelAll:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowAll())
	case LevelDebug:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowDebug())
	case LevelInfo:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowInfo())
	case LevelWarn:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowWarn())
	case LevelError:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowError())
	case LevelNone:
		l = gokitlevel.NewFilter(l, gokitlevel.AllowNone())
	default:
		return nil, fmt.Errorf("unexpected value for log level: %q, see --help", lvl)
	}
	l = gokitlog.With(l,
		"ts", gokitlog.DefaultTimestampUTC,
		"caller", gokitlog.Caller(4),
	)
	return &Logger{l}, nil
}

// With returns a new contextual logger with keyvals prepended to those passed
// to calls to Log.
func With(l Logger, keyvals ...interface{}) *Logger {
	return &Logger{gokitlog.With(l.Logger, keyvals...)}
}

// Info logs at the info level.
func (l *Logger) Info(keyvals ...interface{}) {
	gokitlevel.Info(l.Logger).Log(keyvals...)
}

// ErrorWithBackoff logs at the error level and also blocks if it is called
// quite often (1000 times in a second). This behavior is helpful when it used
// in overly tight hot error loops.
func (l *Logger) ErrorWithBackoff(keyvals ...interface{}) {
	gokitlevel.Error(l.Logger).Log(keyvals...)
	errorBackoff()
}

// Fatal logs the given key values and calls os.Exit(1). This should only be
// used by main() function in package main.
func (l *Logger) Fatal(keyvals ...interface{}) {
	l.Logger.Log(keyvals...)
	os.Exit(1)
}
