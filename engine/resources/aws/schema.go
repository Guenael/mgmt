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
	"os"
	"strings"
)

// CFNSchema represents a CloudFormation resource type schema. This is the
// format returned by `aws cloudformation describe-type`.
type CFNSchema struct {
	TypeName             string                  `json:"typeName"`
	Description          string                  `json:"description"`
	Properties           map[string]*CFNProperty `json:"properties"`
	PrimaryIdentifier    []string                `json:"primaryIdentifier"`
	AdditionalIdentifiers [][]string             `json:"additionalIdentifiers"`
	ReadOnlyProperties   []string                `json:"readOnlyProperties"`
	WriteOnlyProperties  []string                `json:"writeOnlyProperties"`
	CreateOnlyProperties []string                `json:"createOnlyProperties"`
	Required             []string                `json:"required"`
	Definitions          map[string]*CFNProperty `json:"definitions"`
	Handlers             map[string]*CFNHandler  `json:"handlers"`
}

// CFNProperty represents a single property in a CloudFormation schema.
type CFNProperty struct {
	Type                 string                  `json:"type"`
	Ref                  string                  `json:"$ref"`
	Description          string                  `json:"description"`
	Items                *CFNProperty            `json:"items"`
	Properties           map[string]*CFNProperty `json:"properties"`
	AdditionalProperties interface{}             `json:"additionalProperties"`
	Enum                 []interface{}           `json:"enum"`
	Pattern              string                  `json:"pattern"`
	MinLength            *int                    `json:"minLength"`
	MaxLength            *int                    `json:"maxLength"`
	Minimum              *float64                `json:"minimum"`
	Maximum              *float64                `json:"maximum"`
	Default              interface{}             `json:"default"`
	OneOf                []*CFNProperty          `json:"oneOf"`
	AnyOf                []*CFNProperty          `json:"anyOf"`

	// InsertionOrder indicates whether the array preserves insertion order.
	InsertionOrder *bool `json:"insertionOrder"`
}

// CFNHandler describes what operations a CloudFormation resource type supports.
type CFNHandler struct {
	Permissions []string `json:"permissions"`
}

// LoadSchema reads and parses a CloudFormation resource type schema from a JSON
// file.
func LoadSchema(path string) (*CFNSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading schema file %s: %w", path, err)
	}

	schema := &CFNSchema{}
	if err := json.Unmarshal(data, schema); err != nil {
		return nil, fmt.Errorf("error parsing schema file %s: %w", path, err)
	}

	return schema, nil
}

// ReadOnlySet returns a set of top-level property names that are read-only.
// The paths in the schema are like "/properties/Arn".
func (obj *CFNSchema) ReadOnlySet() map[string]bool {
	result := map[string]bool{}
	for _, path := range obj.ReadOnlyProperties {
		name := propertyNameFromPath(path)
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// CreateOnlySet returns a set of top-level property names that are create-only.
func (obj *CFNSchema) CreateOnlySet() map[string]bool {
	result := map[string]bool{}
	for _, path := range obj.CreateOnlyProperties {
		name := propertyNameFromPath(path)
		if name != "" {
			result[name] = true
		}
	}
	return result
}

// RequiredSet returns a set of property names that are required.
func (obj *CFNSchema) RequiredSet() map[string]bool {
	result := map[string]bool{}
	for _, name := range obj.Required {
		result[name] = true
	}
	return result
}

// PrimaryIdentifierNames returns the top-level property names that make up the
// primary identifier.
func (obj *CFNSchema) PrimaryIdentifierNames() []string {
	names := []string{}
	for _, path := range obj.PrimaryIdentifier {
		name := propertyNameFromPath(path)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// SupportsCreate returns true if the schema declares a create handler.
func (obj *CFNSchema) SupportsCreate() bool {
	_, ok := obj.Handlers["create"]
	return ok
}

// SupportsRead returns true if the schema declares a read handler.
func (obj *CFNSchema) SupportsRead() bool {
	_, ok := obj.Handlers["read"]
	return ok
}

// SupportsUpdate returns true if the schema declares an update handler.
func (obj *CFNSchema) SupportsUpdate() bool {
	_, ok := obj.Handlers["update"]
	return ok
}

// SupportsDelete returns true if the schema declares a delete handler.
func (obj *CFNSchema) SupportsDelete() bool {
	_, ok := obj.Handlers["delete"]
	return ok
}

// SupportsList returns true if the schema declares a list handler.
func (obj *CFNSchema) SupportsList() bool {
	_, ok := obj.Handlers["list"]
	return ok
}

// ServiceName returns the AWS service name from the type name. For example,
// "AWS::S3::Bucket" returns "s3".
func (obj *CFNSchema) ServiceName() string {
	parts := strings.Split(obj.TypeName, "::")
	if len(parts) < 3 {
		return ""
	}
	return strings.ToLower(parts[1])
}

// ResourceName returns the resource name from the type name. For example,
// "AWS::S3::Bucket" returns "bucket".
func (obj *CFNSchema) ResourceName() string {
	parts := strings.Split(obj.TypeName, "::")
	if len(parts) < 3 {
		return ""
	}
	return strings.ToLower(parts[2])
}

// MgmtKind returns the mgmt resource kind. For example, "AWS::S3::Bucket"
// returns "aws:s3:bucket".
func (obj *CFNSchema) MgmtKind() string {
	return fmt.Sprintf("aws:%s:%s", obj.ServiceName(), obj.ResourceName())
}

// ResolveRef resolves a $ref path like "#/definitions/Tag" against the schema's
// definitions. It returns a copy of the resolved property.
func (obj *CFNSchema) ResolveRef(ref string) *CFNProperty {
	if !strings.HasPrefix(ref, "#/definitions/") {
		return nil
	}
	name := strings.TrimPrefix(ref, "#/definitions/")
	if def, ok := obj.Definitions[name]; ok {
		return def
	}
	return nil
}

// propertyNameFromPath extracts the property name from a CloudFormation
// property path like "/properties/BucketName".
func propertyNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[1] == "properties" {
		return parts[2]
	}
	return ""
}
