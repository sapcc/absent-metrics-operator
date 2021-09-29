///usr/bin/env go run "$0" "$@"; exit $?

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

// Command releaseinfo parses a changelog that uses the keep-a-changelog
// format, extracts the changes for provided Git tag, and prints the result
// to stdout.
//
// Usage: releaseinfo path-to-changelog vX.Y.Z

//go:build ignore
// +build ignore

package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func handleErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) != 3 {
		handleErr(errors.New("Usage: releaseinfo path-to-changelog vX.Y.Z"))
	}

	tagHeadingRx, err := regexp.Compile(`^## \[(\d{1}\.\d{1}\.\d{1})\] - \d{4}-\d{2}-\d{2}\s*$`)
	handleErr(err)

	file, err := os.Open(os.Args[1])
	handleErr(err)
	defer file.Close()

	var releaseInfo []string
	tag := strings.TrimPrefix(os.Args[2], "v")
	in := false // true if we are inside the given tag's release block
	buf := bufio.NewScanner(file)
	for buf.Scan() {
		line := buf.Text()
		if ml := tagHeadingRx.FindStringSubmatch(line); len(ml) > 0 {
			if in {
				break
			}
			if ml[1] == tag {
				in = true
				continue
			}
		}

		if in {
			if l := strings.TrimSpace(line); len(l) > 0 {
				releaseInfo = append(releaseInfo, l)
			}
		}
	}
	handleErr(buf.Err())

	if len(releaseInfo) == 0 {
		handleErr(fmt.Errorf("could not find release info for tag %q", os.Args[2]))
	}

	fmt.Printf("%s\n", strings.Join(releaseInfo, "\n"))
}
