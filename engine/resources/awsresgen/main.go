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

// This is a code generator that reads CloudFormation resource type schemas and
// generates mgmt resource files that use the AWS Cloud Control API.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (
	schemasDir    = flag.String("schemas", "engine/resources/awsresgen/schemas", "path to CFN schema directory")
	outputDir     = flag.String("output", "engine/resources/aws", "output directory for generated files")
	templates     = flag.String("templates", "engine/resources/awsresgen/templates/aws_resource.go.tpl", "path to the Go template")
	resourcesFile = flag.String("resources", "", "path to resources.txt whitelist (if empty, process all schemas)")
)

func main() {
	flag.Parse()

	// Load the whitelist if provided.
	whitelist, err := loadWhitelist(*resourcesFile)
	if err != nil {
		log.Fatalf("error loading resources file: %v", err)
	}

	// Find all JSON schema files.
	schemaFiles, err := filepath.Glob(filepath.Join(*schemasDir, "*.json"))
	if err != nil {
		log.Fatalf("error finding schemas: %v", err)
	}

	if len(schemaFiles) == 0 {
		log.Fatalf("no schema files found in %s", *schemasDir)
	}

	// Filter schemas against the whitelist by peeking at each file's
	// typeName field. This avoids loading full schemas into memory for
	// types we don't need.
	filtered := []string{}
	for _, f := range schemaFiles {
		typeName, err := peekTypeName(f)
		if err != nil {
			log.Printf("warning: skipping %s: %v", f, err)
			continue
		}
		if whitelist != nil && !whitelist[typeName] {
			continue
		}
		filtered = append(filtered, f)
	}

	if len(filtered) == 0 {
		log.Fatalf("no schemas matched the whitelist in %s", *resourcesFile)
	}

	// Group schemas by service.
	serviceSchemas := map[string][]string{}
	for _, f := range filtered {
		service := serviceFromFilename(filepath.Base(f))
		serviceSchemas[service] = append(serviceSchemas[service], f)
	}

	// Generate one file per service, skipping if the output is already
	// up-to-date (newer than all input schemas and the template).
	generated := 0
	skipped := 0
	for service, files := range serviceSchemas {
		outputFile := filepath.Join(*outputDir, fmt.Sprintf("aws_%s_gen.go", service))

		if isUpToDate(outputFile, files, *templates) {
			skipped++
			continue
		}

		log.Printf("Generating %s from %d schema(s)...", outputFile, len(files))
		if err := generateServiceFile(files, outputFile, *templates); err != nil {
			log.Fatalf("error generating %s: %v", outputFile, err)
		}
		generated++
	}

	if generated == 0 && skipped > 0 {
		log.Printf("All %d service file(s) are up-to-date.", skipped)
	} else {
		log.Printf("Generated %d service file(s), %d up-to-date.", generated, skipped)
	}
}

// serviceFromFilename extracts the service name from a schema filename.
// For example, "AWS_S3_Bucket.json" returns "s3".
func serviceFromFilename(filename string) string {
	name := strings.TrimSuffix(filename, ".json")
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		return strings.ToLower(parts[1])
	}
	return strings.ToLower(name)
}

// loadWhitelist reads a resources.txt file and returns a set of type names.
// Lines starting with # are comments, empty lines are ignored. Returns nil if
// path is empty (meaning process all schemas).
func loadWhitelist(path string) (map[string]bool, error) {
	if path == "" {
		return nil, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error opening %s: %w", path, err)
	}
	defer f.Close()

	result := map[string]bool{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		result[line] = true
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading %s: %w", path, err)
	}

	return result, nil
}

// isUpToDate returns true if outputFile exists and is newer than all input
// schema files and the template file.
func isUpToDate(outputFile string, schemaFiles []string, templateFile string) bool {
	outInfo, err := os.Stat(outputFile)
	if err != nil {
		return false // output doesn't exist
	}
	outTime := outInfo.ModTime()

	for _, f := range schemaFiles {
		info, err := os.Stat(f)
		if err != nil {
			return false
		}
		if info.ModTime().After(outTime) {
			return false // schema is newer
		}
	}

	tplInfo, err := os.Stat(templateFile)
	if err != nil {
		return false
	}
	if tplInfo.ModTime().After(outTime) {
		return false // template is newer
	}

	return true
}

// typeNameOnly is a minimal struct for peeking at a schema's typeName without
// loading the full schema.
type typeNameOnly struct {
	TypeName string `json:"typeName"`
}

// peekTypeName reads just the typeName field from a schema file without
// parsing the entire document.
func peekTypeName(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var t typeNameOnly
	if err := json.Unmarshal(data, &t); err != nil {
		return "", err
	}

	if t.TypeName == "" {
		return "", fmt.Errorf("no typeName in %s", path)
	}

	return t.TypeName, nil
}
