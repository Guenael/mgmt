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

package lang

import (
	"strings"
	"testing"

	"github.com/purpleidea/mgmt/lang/ast"
	"github.com/purpleidea/mgmt/lang/parser"
)

// parseAndFormat is a test helper that parses MCL source and formats it.
func parseAndFormat(t *testing.T, input string) string {
	t.Helper()
	comments, tree, err := parser.LexParseWithComments(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse error: %s\ninput:\n%s", err, input)
	}
	// Convert parser.Comment to ast.CommentData.
	cmts := make([]*ast.CommentData, len(comments))
	for i, c := range comments {
		cmts[i] = &ast.CommentData{
			Value:  c.Value,
			Row:    c.Row,
			Col:    c.Col,
			Inline: c.Inline,
		}
	}
	return ast.FormatWithComments(tree, cmts)
}

func TestFormatSimpleBind(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "string bind",
			input:  `$x = "hello"`,
			expect: `$x = "hello"`,
		},
		{
			name:   "int bind",
			input:  `$x = 42`,
			expect: `$x = 42`,
		},
		{
			name:   "float bind",
			input:  `$x = 3.14`,
			expect: `$x = 3.14`,
		},
		{
			name:   "bool bind",
			input:  `$x = true`,
			expect: `$x = true`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatResource(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name: "simple resource",
			input: `test "t1" {
	stringptr => "hello",
}`,
			expect: `test "t1" {
	stringptr => "hello",
}`,
		},
		{
			name: "resource with multiple fields",
			input: `test "t1" {
	stringptr => "hello",
	int64ptr => 42,
}`,
			expect: `test "t1" {
	stringptr => "hello",
	int64ptr => 42,
}`,
		},
		{
			name: "resource with meta",
			input: `test "t1" {
	stringptr => "hello",
	Meta:noop => true,
}`,
			expect: `test "t1" {
	stringptr => "hello",
	Meta:noop => true,
}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:\n%s\nexpect:\n%s", got, tt.expect)
			}
		})
	}
}

func TestFormatComments(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "top-level comment",
			input:  "# hello world",
			expect: "# hello world",
		},
		{
			name:   "comment before resource",
			input:  "# a comment\ntest \"t1\" {\n\tstringptr => \"hi\",\n}",
			expect: "# a comment\ntest \"t1\" {\n\tstringptr => \"hi\",\n}",
		},
		{
			name:   "comment inside resource",
			input:  "test \"t1\" {\n\t# field comment\n\tstringptr => \"hi\",\n}",
			expect: "test \"t1\" {\n\t# field comment\n\tstringptr => \"hi\",\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatIfStatement(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple if",
			input:  "if true {\n\t$x = 1\n}",
			expect: "if true {\n\t$x = 1\n}",
		},
		{
			name:   "if else",
			input:  "if true {\n\t$x = 1\n} else {\n\t$x = 2\n}",
			expect: "if true {\n\t$x = 1\n} else {\n\t$x = 2\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:\n%s\nexpect:\n%s", got, tt.expect)
			}
		})
	}
}

func TestFormatOperators(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "addition",
			input:  "$x = 1 + 2",
			expect: "$x = 1 + 2",
		},
		{
			name:   "comparison",
			input:  "$x = 1 > 2",
			expect: "$x = 1 > 2",
		},
		{
			name:   "logical and",
			input:  "$x = true and false",
			expect: "$x = true and false",
		},
		{
			name:   "not",
			input:  "$x = not true",
			expect: "$x = not true",
		},
		{
			name:   "nested operators with precedence",
			input:  "$x = 1 + 2 * 3",
			expect: "$x = 1 + 2 * 3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatExpressions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "list",
			input:  `$x = [1, 2, 3,]`,
			expect: `$x = [1, 2, 3,]`,
		},
		{
			name:   "empty list",
			input:  `$x = []`,
			expect: `$x = []`,
		},
		{
			name:   "map",
			input:  `$x = {"a" => 1, "b" => 2,}`,
			expect: `$x = {"a" => 1, "b" => 2,}`,
		},
		{
			name:   "empty map",
			input:  `$x = {}`,
			expect: `$x = {}`,
		},
		{
			name:   "struct",
			input:  `$x = struct{name => "foo", age => 42,}`,
			expect: `$x = struct{name => "foo", age => 42,}`,
		},
		{
			name:   "variable",
			input:  "$x = $y",
			expect: "$x = $y",
		},
		{
			name:   "if expression",
			input:  `$x = if true { "yes" } else { "no" }`,
			expect: `$x = if true { "yes" } else { "no" }`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatFunctions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "lambda",
			input:  `$fn = func($x) { $x + 1 }`,
			expect: `$fn = func($x) { $x + 1 }`,
		},
		{
			name:   "function call",
			input:  `$x = len("hello")`,
			expect: `$x = len("hello")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatLookups(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "list index",
			input:  `$x = $list[0]`,
			expect: `$x = $list[0]`,
		},
		{
			name:   "map lookup with default",
			input:  `$x = $map["key"] || "default"`,
			expect: `$x = $map["key"] || "default"`,
		},
		{
			name:   "struct field",
			input:  `$x = $s->name`,
			expect: `$x = $s->name`,
		},
		{
			name:   "struct field with default",
			input:  `$x = $s->name || "default"`,
			expect: `$x = $s->name || "default"`,
		},
		{
			name:   "membership",
			input:  `$x = "a" in $list`,
			expect: `$x = "a" in $list`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatImportInclude(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "import",
			input:  `import "fmt"`,
			expect: `import "fmt"`,
		},
		{
			name:   "import with alias",
			input:  `import "golang/strings" as golang_strings`,
			expect: `import "golang/strings" as golang_strings`,
		},
		{
			name:   "include",
			input:  `include myclass`,
			expect: `include myclass`,
		},
		{
			name:   "include with args",
			input:  `include greeter("World")`,
			expect: `include greeter("World")`,
		},
		{
			name:   "include with alias",
			input:  `include myclass as inst1`,
			expect: `include myclass as inst1`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatEdges(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple edge",
			input:  `File["/etc/config"] -> Svc["nginx"]`,
			expect: `File["/etc/config"] -> Svc["nginx"]`,
		},
		{
			name:   "multi-hop edge",
			input:  `Exec["a"] -> Exec["b"] -> Exec["c"]`,
			expect: `Exec["a"] -> Exec["b"] -> Exec["c"]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatLoops(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "for loop",
			input:  "for $i, $v in $list {\n\t$x = $v\n}",
			expect: "for $i, $v in $list {\n\t$x = $v\n}",
		},
		{
			name:   "forkv loop",
			input:  "forkv $k, $v in $map {\n\t$x = $v\n}",
			expect: "forkv $k, $v in $map {\n\t$x = $v\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:\n%s\nexpect:\n%s", got, tt.expect)
			}
		})
	}
}

func TestFormatClassFunc(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple class",
			input:  "class myclass {\n\t$x = 1\n}",
			expect: "class myclass {\n\t$x = 1\n}",
		},
		{
			name:   "class with args",
			input:  "class myclass($a, $b) {\n\t$x = $a\n}",
			expect: "class myclass($a, $b) {\n\t$x = $a\n}",
		},
		{
			name:   "named func",
			input:  "func myfunc($a, $b) {\n\t$a + $b\n}",
			expect: "func myfunc($a, $b) {\n\t$a + $b\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:\n%s\nexpect:\n%s", got, tt.expect)
			}
		})
	}
}

func TestFormatPanic(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "simple panic",
			input:  `panic("something went wrong")`,
			expect: `panic("something went wrong")`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

func TestFormatBlankLines(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "single blank line preserved",
			input:  "$x = 1\n\n$y = 2",
			expect: "$x = 1\n\n$y = 2",
		},
		{
			name:   "multiple blank lines collapsed to one",
			input:  "$x = 1\n\n\n\n$y = 2",
			expect: "$x = 1\n\n$y = 2",
		},
		{
			name:   "no blank line stays as no blank line",
			input:  "$x = 1\n$y = 2",
			expect: "$x = 1\n$y = 2",
		},
		{
			name:   "blank line between import and resource",
			input:  "import \"fmt\"\n\n$x = 42",
			expect: "import \"fmt\"\n\n$x = 42",
		},
		{
			name:   "blank line between resources",
			input:  "test \"t1\" {\n\tstringptr => \"a\",\n}\n\ntest \"t2\" {\n\tstringptr => \"b\",\n}",
			expect: "test \"t1\" {\n\tstringptr => \"a\",\n}\n\ntest \"t2\" {\n\tstringptr => \"b\",\n}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAndFormat(t, tt.input)
			if got != tt.expect {
				t.Errorf("format mismatch\ngot:    %q\nexpect: %q", got, tt.expect)
			}
		})
	}
}

// TestFormatIdempotent verifies that formatting already-formatted code produces
// identical output.
func TestFormatIdempotent(t *testing.T) {
	inputs := []string{
		`$x = "hello"`,
		"test \"t1\" {\n\tstringptr => \"hello\",\n}",
		"if true {\n\t$x = 1\n} else {\n\t$x = 2\n}",
		"for $i, $v in $list {\n\t$x = $v\n}",
		"class myclass($a) {\n\t$x = $a\n}",
		`import "fmt"`,
		`include myclass("arg")`,
		"# a comment\n$x = 42",
		`$x = 1 + 2 * 3`,
		`$x = $list[0] || "default"`,
		"$x = 1\n\n$y = 2",
	}
	for _, input := range inputs {
		first := parseAndFormat(t, input)
		second := parseAndFormat(t, first)
		if first != second {
			t.Errorf("not idempotent\ninput:\n%s\nfirst:\n%s\nsecond:\n%s", input, first, second)
		}
	}
}
