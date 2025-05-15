// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"strconv"
)

// parseBool is a wrapper around strconv.ParseBool() that returns false in case of an
// error.
func parseBool(str string) bool {
	v, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return v
}
