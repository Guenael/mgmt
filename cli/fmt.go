// Mgmt
// Copyright (C) 2013-2024+ James Shubin and the project contributors
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

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cliUtil "github.com/purpleidea/mgmt/cli/util"
)

// FmtArgs is the CLI parsing structure and type of the parsed result for the
// `fmt` subcommand.
type FmtArgs struct {
	// Input is the input mcl file or directory path to format.
	Input string `arg:"positional,required" help:"input mcl file or directory path"`

	// Write causes the formatted output to be written back to the source
	// file(s) instead of being printed to stdout.
	Write bool `arg:"-w,--write" help:"write result to (source) file instead of stdout"`

	// List causes only the file paths of unformatted files to be printed.
	List bool `arg:"-l,--list" help:"list files whose formatting differs"`
}

// Run executes the fmt subcommand.
func (obj *FmtArgs) Run(ctx context.Context, data *cliUtil.Data) (bool, error) {
	info, err := os.Stat(obj.Input)
	if err != nil {
		return true, fmt.Errorf("cannot access input: %s", err)
	}

	var files []string
	if info.IsDir() {
		err := filepath.Walk(obj.Input, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !fi.IsDir() && strings.HasSuffix(path, ".mcl") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return true, err
		}
	} else {
		files = []string{obj.Input}
	}

	hasUnformatted := false
	for _, path := range files {
		formatted, original, err := formatFile(path)
		if err != nil {
			return true, fmt.Errorf("error formatting %s: %s", path, err)
		}

		if obj.List {
			if formatted != original {
				fmt.Println(path)
				hasUnformatted = true
			}
			continue
		}

		if obj.Write {
			if formatted != original {
				if err := os.WriteFile(path, []byte(formatted), 0644); err != nil {
					return true, fmt.Errorf("error writing %s: %s", path, err)
				}
			}
			continue
		}

		// Default: print formatted output to stdout.
		fmt.Print(formatted)
	}

	if obj.List && hasUnformatted {
		return true, fmt.Errorf("found unformatted files")
	}

	return true, nil
}

// formatFile reads an MCL file, parses it, formats it, and returns both the
// formatted output and the original content.
func formatFile(path string) (formatted string, original string, err error) {
	if cliUtil.FormatFunc == nil {
		return "", "", fmt.Errorf("formatter not registered")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	original = string(data)

	formatted, err = cliUtil.FormatFunc(original)
	if err != nil {
		return "", "", err
	}
	return formatted, original, nil
}
