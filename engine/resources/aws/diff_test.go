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
	"testing"
)

func TestComputePatchNoChanges(t *testing.T) {
	current := map[string]interface{}{
		"Name":   "test",
		"Region": "us-east-1",
	}
	desired := map[string]interface{}{
		"Name":   "test",
		"Region": "us-east-1",
	}
	ops := ComputePatch(current, desired, nil)
	if len(ops) != 0 {
		t.Errorf("expected no ops, got %d", len(ops))
	}
}

func TestComputePatchAdd(t *testing.T) {
	current := map[string]interface{}{
		"Name": "test",
	}
	desired := map[string]interface{}{
		"Name":   "test",
		"Region": "us-east-1",
	}
	ops := ComputePatch(current, desired, nil)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Op != "add" {
		t.Errorf("expected add, got %s", ops[0].Op)
	}
	if ops[0].Path != "/Region" {
		t.Errorf("expected /Region, got %s", ops[0].Path)
	}
}

func TestComputePatchReplace(t *testing.T) {
	current := map[string]interface{}{
		"Name": "old",
	}
	desired := map[string]interface{}{
		"Name": "new",
	}
	ops := ComputePatch(current, desired, nil)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Op != "replace" {
		t.Errorf("expected replace, got %s", ops[0].Op)
	}
}

func TestComputePatchRemove(t *testing.T) {
	current := map[string]interface{}{
		"Name":  "test",
		"Extra": "value",
	}
	desired := map[string]interface{}{
		"Name": "test",
	}
	ops := ComputePatch(current, desired, nil)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Op != "remove" {
		t.Errorf("expected remove, got %s", ops[0].Op)
	}
	if ops[0].Path != "/Extra" {
		t.Errorf("expected /Extra, got %s", ops[0].Path)
	}
}

func TestComputePatchSkipsReadOnly(t *testing.T) {
	current := map[string]interface{}{
		"Name": "test",
		"Arn":  "arn:aws:s3:::test",
	}
	desired := map[string]interface{}{
		"Name": "test",
	}
	readOnly := map[string]bool{"Arn": true}
	ops := ComputePatch(current, desired, readOnly)
	if len(ops) != 0 {
		t.Errorf("expected no ops (Arn is read-only), got %d", len(ops))
	}
}

func TestComputePatchNestedObject(t *testing.T) {
	current := map[string]interface{}{
		"Config": map[string]interface{}{
			"Status": "Enabled",
		},
	}
	desired := map[string]interface{}{
		"Config": map[string]interface{}{
			"Status": "Suspended",
		},
	}
	ops := ComputePatch(current, desired, nil)
	if len(ops) != 1 {
		t.Fatalf("expected 1 op, got %d", len(ops))
	}
	if ops[0].Op != "replace" {
		t.Errorf("expected replace, got %s", ops[0].Op)
	}
}

func TestJsonEqualNumericTypes(t *testing.T) {
	// JSON numbers from unmarshaling are float64, but desired might be int.
	if !jsonEqual(float64(42), float64(42)) {
		t.Error("expected equal")
	}
	// Different values should not be equal.
	if jsonEqual(float64(42), float64(43)) {
		t.Error("expected not equal")
	}
}
