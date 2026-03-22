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
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

var (
	schemasDir = flag.String("schemas", "engine/resources/awsresgen/schemas", "path to CFN schema directory")
	outputDir  = flag.String("output", "engine/resources", "output directory for generated files")
	templates  = flag.String("templates", "engine/resources/awsresgen/templates/aws_resource.go.tpl", "path to the Go template")
)

func main() {
	flag.Parse()

	// Find all JSON schema files.
	schemaFiles, err := filepath.Glob(filepath.Join(*schemasDir, "*.json"))
	if err != nil {
		log.Fatalf("error finding schemas: %v", err)
	}

	if len(schemaFiles) == 0 {
		log.Fatalf("no schema files found in %s", *schemasDir)
	}

	// Group schemas by service.
	serviceSchemas := map[string][]string{} // service -> list of schema file paths
	for _, f := range schemaFiles {
		service := serviceFromFilename(filepath.Base(f))
		serviceSchemas[service] = append(serviceSchemas[service], f)
	}

	// Generate one file per service.
	for service, files := range serviceSchemas {
		outputFile := filepath.Join(*outputDir, fmt.Sprintf("aws_%s_gen.go", service))
		log.Printf("Generating %s from %d schema(s)...", outputFile, len(files))

		if err := generateServiceFile(files, outputFile, *templates); err != nil {
			log.Fatalf("error generating %s: %v", outputFile, err)
		}
	}

	log.Printf("Done. Generated %d service file(s).", len(serviceSchemas))
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
