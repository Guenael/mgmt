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

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"unicode"
)

// ResourceData holds all the data needed to generate a resource from a
// template.
type ResourceData struct {
	// TypeName is the CloudFormation type name (e.g., "AWS::S3::Bucket").
	TypeName string

	// MgmtKind is the mgmt resource kind (e.g., "aws:s3:bucket").
	MgmtKind string

	// GoStructName is the Go struct name (e.g., "AwsS3BucketRes").
	GoStructName string

	// GoStructPrefix is the Go struct prefix without "Res" (e.g.,
	// "AwsS3Bucket").
	GoStructPrefix string

	// Description is the resource description from the schema.
	Description string

	// Properties is the list of writable properties.
	Properties []PropertyData

	// ReadOnlyProperties is the list of read-only properties.
	ReadOnlyProperties []PropertyData

	// NestedStructs is the list of nested struct definitions needed.
	NestedStructs []NestedStructData

	// PrimaryIdentifierFields is the list of Go field names that make up
	// the primary identifier.
	PrimaryIdentifierFields []string

	// RequiredFields is the set of Go field names that are required.
	RequiredFields map[string]bool

	// CreateOnlyFields is the set of Go field names that are create-only.
	CreateOnlyFields map[string]bool

	// SupportsUpdate is true if the schema declares an update handler.
	SupportsUpdate bool
}

// PropertyData holds data about a single resource property.
type PropertyData struct {
	// CFNName is the CloudFormation property name (e.g., "BucketName").
	CFNName string

	// GoName is the Go field name (e.g., "BucketName").
	GoName string

	// LangTag is the mcl lang tag (e.g., "bucketname").
	LangTag string

	// GoType is the Go type string (e.g., "string", "*AwsS3BucketTag").
	GoType string

	// JSONTag is the JSON tag for marshaling to Cloud Control API.
	JSONTag string

	// Description is the property description from the schema.
	Description string

	// IsPointer indicates the field should be a pointer (for optional
	// nested structs).
	IsPointer bool

	// IsReadOnly indicates this property is read-only.
	IsReadOnly bool

	// IsRequired indicates this property is required.
	IsRequired bool

	// IsCreateOnly indicates this property cannot be updated.
	IsCreateOnly bool

	// HasEnum indicates this property has enum constraints.
	HasEnum bool

	// EnumValues is the list of allowed values if HasEnum is true.
	EnumValues []string
}

// NestedStructData holds data for a nested struct type.
type NestedStructData struct {
	// GoName is the struct name (e.g., "AwsS3BucketVersioningConfiguration").
	GoName string

	// Description is the struct description.
	Description string

	// Properties is the list of fields in this struct.
	Properties []PropertyData
}

// ServiceFileData holds data for generating a complete service file.
type ServiceFileData struct {
	// Resources is the list of resources in this service file.
	Resources []ResourceData
}

// generateServiceFile generates a Go source file for a set of schema files
// belonging to the same AWS service.
func generateServiceFile(schemaFiles []string, outputFile, templatePath string) error {
	data := ServiceFileData{}

	for _, schemaFile := range schemaFiles {
		rd, err := processSchema(schemaFile)
		if err != nil {
			return fmt.Errorf("error processing %s: %w", schemaFile, err)
		}
		data.Resources = append(data.Resources, *rd)
	}

	// Sort resources by kind for deterministic output.
	sort.Slice(data.Resources, func(i, j int) bool {
		return data.Resources[i].MgmtKind < data.Resources[j].MgmtKind
	})

	tmpl, err := template.New("aws_resource.go.tpl").Funcs(template.FuncMap{
		"lower":     strings.ToLower,
		"title":     strings.Title,
		"hasPrefix": strings.HasPrefix,
		"join":      strings.Join,
		"quote":     func(s string) string { return fmt.Sprintf("%q", s) },
	}).ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating output file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("error executing template: %w", err)
	}

	// Run gofmt on the output.
	cmd := exec.Command("gofmt", "-s", "-w", outputFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gofmt error: %s: %w", string(out), err)
	}

	return nil
}

// processSchema reads a CFN schema file and produces a ResourceData.
func processSchema(schemaFile string) (*ResourceData, error) {
	raw, err := os.ReadFile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", schemaFile, err)
	}

	var schema cfnSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("error parsing %s: %w", schemaFile, err)
	}

	parts := strings.Split(schema.TypeName, "::")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid type name: %s", schema.TypeName)
	}

	service := strings.ToLower(parts[1])
	resource := parts[2]
	prefix := fmt.Sprintf("Aws%s%s", capitalize(service), resource)

	mgmtKind := fmt.Sprintf("aws:%s:%s", service, strings.ToLower(resource))

	readOnly := toSet(schema.ReadOnlyProperties)
	createOnly := toSet(schema.CreateOnlyProperties)
	required := map[string]bool{}
	for _, r := range schema.Required {
		required[r] = true
	}

	rd := &ResourceData{
		TypeName:        schema.TypeName,
		MgmtKind:        mgmtKind,
		GoStructName:    prefix + "Res",
		GoStructPrefix:  prefix,
		Description:     sanitizeDescription(schema.Description),
		RequiredFields:  map[string]bool{},
		CreateOnlyFields: map[string]bool{},
		SupportsUpdate:  schema.supportsUpdate(),
	}

	// Collect nested structs.
	nestedStructs := map[string]*NestedStructData{}

	// Sort property names for deterministic output.
	propNames := sortedMapKeys(schema.Properties)

	for _, name := range propNames {
		prop := schema.Properties[name]
		propPath := "/properties/" + name
		isRO := readOnly[propPath]
		isCO := createOnly[propPath]
		isReq := required[name]

		goType, nested := resolveGoType(prop, &schema, prefix+name)
		for _, ns := range nested {
			nestedStructs[ns.GoName] = &ns
		}

		goName := name
		langTag := strings.ToLower(name)
		// Avoid collisions with mgmt meta fields and trait methods.
		if goName == "State" || goName == "Region" || goName == "Name" || goName == "Kind" {
			goName = "Aws" + goName
			langTag = "aws" + langTag
		}

		pd := PropertyData{
			CFNName:      name,
			GoName:       goName,
			LangTag:      langTag,
			GoType:       goType,
			JSONTag:      name + ",omitempty",
			Description:  sanitizeDescription(prop.Description),
			IsPointer:    strings.HasPrefix(goType, "*"),
			IsReadOnly:   isRO,
			IsRequired:   isReq,
			IsCreateOnly: isCO,
		}

		if len(prop.Enum) > 0 {
			pd.HasEnum = true
			for _, v := range prop.Enum {
				pd.EnumValues = append(pd.EnumValues, fmt.Sprintf("%v", v))
			}
		}

		if isRO {
			rd.ReadOnlyProperties = append(rd.ReadOnlyProperties, pd)
		} else {
			rd.Properties = append(rd.Properties, pd)
		}

		if isReq {
			rd.RequiredFields[name] = true
		}
		if isCO {
			rd.CreateOnlyFields[name] = true
		}
	}

	// Primary identifier fields.
	for _, path := range schema.PrimaryIdentifier {
		name := propertyNameFromPath(path)
		if name != "" {
			rd.PrimaryIdentifierFields = append(rd.PrimaryIdentifierFields, name)
		}
	}

	// Collect nested structs in sorted order.
	nsNames := sortedMapKeys(nestedStructs)
	for _, name := range nsNames {
		rd.NestedStructs = append(rd.NestedStructs, *nestedStructs[name])
	}

	return rd, nil
}

// cfnSchema is a simplified CloudFormation resource type schema for the
// generator. We don't import the aws package here since this is a standalone
// tool.
type cfnSchema struct {
	TypeName             string                  `json:"typeName"`
	Description          string                  `json:"description"`
	Properties           map[string]*cfnProperty `json:"properties"`
	PrimaryIdentifier    []string                `json:"primaryIdentifier"`
	ReadOnlyProperties   []string                `json:"readOnlyProperties"`
	CreateOnlyProperties []string                `json:"createOnlyProperties"`
	Required             []string                `json:"required"`
	Definitions          map[string]*cfnProperty `json:"definitions"`
	Handlers             map[string]interface{}  `json:"handlers"`
}

func (obj *cfnSchema) supportsUpdate() bool {
	_, ok := obj.Handlers["update"]
	return ok
}

// flexString handles JSON fields that can be a string or an array of strings.
// In CFN schemas, "type" can be "string" or ["string", "object"].
type flexString string

func (obj *flexString) UnmarshalJSON(data []byte) error {
	// Try string first.
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*obj = flexString(s)
		return nil
	}
	// Try array of strings - take the first element.
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) > 0 {
			*obj = flexString(arr[0])
		}
		return nil
	}
	return fmt.Errorf("cannot unmarshal %s into flexString", string(data))
}

type cfnProperty struct {
	Type        flexString              `json:"type"`
	Ref         string                  `json:"$ref"`
	Description string                  `json:"description"`
	Items       *cfnProperty            `json:"items"`
	Properties  map[string]*cfnProperty `json:"properties"`
	Enum        []interface{}           `json:"enum"`
	Pattern     string                  `json:"pattern"`
	MinLength   *int                    `json:"minLength"`
	MaxLength   *int                    `json:"maxLength"`
	Minimum     *float64                `json:"minimum"`
	Maximum     *float64                `json:"maximum"`
	Default     interface{}             `json:"default"`
	OneOf       []*cfnProperty          `json:"oneOf"`
	AnyOf       []*cfnProperty          `json:"anyOf"`
}

// resolveGoType converts a CFN property to a Go type string and collects
// nested struct definitions.
func resolveGoType(prop *cfnProperty, schema *cfnSchema, prefix string) (string, []NestedStructData) {
	if prop == nil {
		return "interface{}", nil
	}

	// Resolve $ref.
	if prop.Ref != "" {
		refName := strings.TrimPrefix(prop.Ref, "#/definitions/")
		if def, ok := schema.Definitions[refName]; ok {
			return resolveGoType(def, schema, prefix)
		}
		return "interface{}", nil
	}

	switch string(prop.Type) {
	case "string":
		return "string", nil
	case "integer":
		return "int64", nil
	case "boolean":
		return "bool", nil
	case "number":
		return "float64", nil
	case "array":
		if prop.Items != nil {
			elemType, nested := resolveGoType(prop.Items, schema, prefix+"Item")
			// If element is a pointer struct, use without pointer in slice.
			elemType = strings.TrimPrefix(elemType, "*")
			return "[]" + elemType, nested
		}
		return "[]interface{}", nil
	case "object":
		if prop.Properties != nil && len(prop.Properties) > 0 {
			ns, nested := buildNestedStruct(prop, schema, prefix)
			nested = append(nested, ns)
			return "*" + prefix, nested
		}
		return "map[string]interface{}", nil
	default:
		// No explicit type. Check if has properties (treat as object).
		if prop.Properties != nil && len(prop.Properties) > 0 {
			ns, nested := buildNestedStruct(prop, schema, prefix)
			nested = append(nested, ns)
			return "*" + prefix, nested
		}
		// Check for oneOf/anyOf - use interface{}.
		if len(prop.OneOf) > 0 || len(prop.AnyOf) > 0 {
			return "interface{}", nil
		}
		return "interface{}", nil
	}
}

// buildNestedStruct creates a NestedStructData for an object property.
func buildNestedStruct(prop *cfnProperty, schema *cfnSchema, name string) (NestedStructData, []NestedStructData) {
	ns := NestedStructData{
		GoName:      name,
		Description: sanitizeDescription(prop.Description),
	}
	var allNested []NestedStructData

	propNames := sortedMapKeys(prop.Properties)
	for _, pName := range propNames {
		p := prop.Properties[pName]
		goType, nested := resolveGoType(p, schema, name+pName)
		allNested = append(allNested, nested...)

		ns.Properties = append(ns.Properties, PropertyData{
			CFNName:     pName,
			GoName:      pName,
			LangTag:     strings.ToLower(pName),
			GoType:      goType,
			JSONTag:     pName + ",omitempty",
			Description: sanitizeDescription(p.Description),
			IsPointer:   strings.HasPrefix(goType, "*"),
		})
	}

	return ns, allNested
}

// propertyNameFromPath extracts the property name from a CFN path.
func propertyNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[1] == "properties" {
		return parts[2]
	}
	return ""
}

// toSet converts a list of "/properties/Name" paths to a set.
func toSet(paths []string) map[string]bool {
	result := map[string]bool{}
	for _, p := range paths {
		result[p] = true
	}
	return result
}

// sanitizeDescription cleans a CFN description for use in Go comments. It
// replaces newlines with spaces and trims to a reasonable length.
func sanitizeDescription(s string) string {
	// Replace all kinds of whitespace (newlines, tabs) with spaces.
	s = strings.Join(strings.Fields(s), " ")
	// Truncate very long descriptions.
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// capitalize returns a string with the first letter uppercased.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// sortedMapKeys returns the sorted keys of a map.
func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
