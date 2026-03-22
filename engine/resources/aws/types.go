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
	"strings"
)

// GoType represents a Go type that was resolved from a CFN property.
type GoType struct {
	// Name is the Go type name (e.g., "string", "int64", "[]string",
	// "*MyStruct").
	Name string

	// IsPointer indicates that the type should be a pointer (nullable).
	IsPointer bool

	// IsSlice indicates that the type is a slice.
	IsSlice bool

	// IsMap indicates that the type is a map[string]interface{}.
	IsMap bool

	// StructName is set when the type is a generated struct.
	StructName string

	// ElementType is the inner type for slices.
	ElementType *GoType
}

// String returns the Go type string.
func (obj *GoType) String() string {
	if obj.IsPointer {
		return "*" + obj.Name
	}
	return obj.Name
}

// ResolveGoType converts a CFN property type to a Go type. The prefix is used
// for naming nested struct types (e.g., "AwsS3Bucket").
func ResolveGoType(prop *CFNProperty, schema *CFNSchema, prefix string) *GoType {
	if prop == nil {
		return &GoType{Name: "interface{}"}
	}

	// Resolve $ref first.
	if prop.Ref != "" {
		resolved := schema.ResolveRef(prop.Ref)
		if resolved != nil {
			return ResolveGoType(resolved, schema, prefix)
		}
		return &GoType{Name: "interface{}"}
	}

	switch prop.Type {
	case "string":
		return &GoType{Name: "string"}
	case "integer":
		return &GoType{Name: "int64"}
	case "boolean":
		return &GoType{Name: "bool"}
	case "number":
		return &GoType{Name: "float64"}
	case "array":
		if prop.Items != nil {
			elemType := ResolveGoType(prop.Items, schema, prefix)
			return &GoType{
				Name:        "[]" + elemType.String(),
				IsSlice:     true,
				ElementType: elemType,
			}
		}
		return &GoType{Name: "[]interface{}"}
	case "object":
		if prop.Properties != nil && len(prop.Properties) > 0 {
			return &GoType{
				Name:       prefix,
				IsPointer:  true,
				StructName: prefix,
			}
		}
		return &GoType{Name: "map[string]interface{}", IsMap: true}
	default:
		// If no type but has properties, treat as struct.
		if prop.Properties != nil && len(prop.Properties) > 0 {
			return &GoType{
				Name:       prefix,
				IsPointer:  true,
				StructName: prefix,
			}
		}
		return &GoType{Name: "interface{}"}
	}
}

// StructToMap converts a struct to a map[string]interface{} by marshaling to
// JSON and back. This is used to build the desired state for Cloud Control API
// calls.
func StructToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("error marshaling to JSON: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("error unmarshaling from JSON: %w", err)
	}

	return result, nil
}

// MapToStruct populates a struct from a map[string]interface{} by marshaling
// through JSON. This is used to populate read-only fields from the Get
// response.
func MapToStruct(m map[string]interface{}, v interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("error marshaling map: %w", err)
	}

	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("error unmarshaling to struct: %w", err)
	}

	return nil
}

// CFNTypeToMgmtKind converts a CloudFormation type name like "AWS::S3::Bucket"
// to an mgmt kind like "aws:s3:bucket".
func CFNTypeToMgmtKind(typeName string) string {
	parts := strings.Split(typeName, "::")
	if len(parts) != 3 {
		return strings.ToLower(typeName)
	}
	return fmt.Sprintf("%s:%s:%s",
		strings.ToLower(parts[0]),
		strings.ToLower(parts[1]),
		strings.ToLower(parts[2]),
	)
}

// MgmtKindToCFNType converts an mgmt kind like "aws:s3:bucket" to a
// CloudFormation type name like "AWS::S3::Bucket".
func MgmtKindToCFNType(kind string) string {
	parts := strings.Split(kind, ":")
	if len(parts) != 3 {
		return kind
	}
	return fmt.Sprintf("%s::%s::%s",
		strings.ToUpper(parts[0]),
		capitalizeFirst(parts[1]),
		capitalizeFirst(parts[2]),
	)
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
