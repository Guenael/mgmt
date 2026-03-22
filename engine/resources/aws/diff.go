// Mgmt
// Copyright (C) James Shubin and the project contributors
// Written by James Shubin <james@shubin.ca> and the project contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.
//
// Additional permission under GNU GPL version 3 section 7
//
// If you modify this program, or any covered work, by linking or combining it
// with embedded mcl code and modules (and that the embedded mcl code and
// modules which link with this program, contain a copy of their source code in
// the authoritative form) containing parts covered by the terms of any other
// license, the licensors of this program grant you additional permission to
// convey the resulting work. Furthermore, the licensors of this program grant
// the original author, James Shubin, additional permission to update this
// additional permission if he deems it necessary to achieve the goals of this
// additional permission.

//go:build !noaws

package aws

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
)

// PatchOp represents one JSON Patch (RFC 6902) operation.
type PatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// ComputePatch compares current and desired property maps and returns a list of
// JSON Patch operations needed to transform current into desired. It skips any
// properties listed in readOnly.
func ComputePatch(current, desired map[string]interface{}, readOnly map[string]bool) []PatchOp {
	ops := []PatchOp{}

	// Find properties to add or replace.
	keys := sortedKeys(desired)
	for _, key := range keys {
		if readOnly[key] {
			continue
		}
		desiredVal := desired[key]
		currentVal, exists := current[key]
		path := fmt.Sprintf("/%s", key)

		if !exists {
			ops = append(ops, PatchOp{
				Op:    "add",
				Path:  path,
				Value: desiredVal,
			})
			continue
		}

		if !jsonEqual(currentVal, desiredVal) {
			ops = append(ops, PatchOp{
				Op:    "replace",
				Path:  path,
				Value: desiredVal,
			})
		}
	}

	// Find properties to remove (in current but not in desired).
	currentKeys := sortedKeys(current)
	for _, key := range currentKeys {
		if readOnly[key] {
			continue
		}
		if _, exists := desired[key]; !exists {
			ops = append(ops, PatchOp{
				Op:   "remove",
				Path: fmt.Sprintf("/%s", key),
			})
		}
	}

	return ops
}

// jsonEqual compares two values for equality by normalizing through JSON. This
// handles cases where numeric types differ (float64 vs int) but represent the
// same JSON value.
func jsonEqual(a, b interface{}) bool {
	// Fast path for simple equality.
	if reflect.DeepEqual(a, b) {
		return true
	}

	// Normalize through JSON to handle type differences.
	aj, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bj, err := json.Marshal(b)
	if err != nil {
		return false
	}

	return string(aj) == string(bj)
}

// sortedKeys returns the keys of a map in sorted order.
func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
