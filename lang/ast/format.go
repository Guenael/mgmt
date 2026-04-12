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

package ast

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/purpleidea/mgmt/lang/funcs"
	"github.com/purpleidea/mgmt/lang/funcs/operators"
	"github.com/purpleidea/mgmt/lang/interfaces"
)

// fmtIndent returns a string of tabs for the given indentation depth.
func fmtIndent(depth int) string {
	return strings.Repeat("\t", depth)
}

// operatorPrecedence returns the precedence level for an operator string. Higher
// values mean higher precedence (tighter binding). This must match the
// precedence table in parser.y.
func operatorPrecedence(op string) int {
	switch op {
	case "and", "or":
		return 1
	case "<", ">", "<=", ">=", "==", "!=":
		return 2
	case "+", "-":
		return 3
	case "*", "/":
		return 4
	case "not":
		return 5
	default:
		return 0
	}
}

// formatExpr formats an expression. This is a helper that dispatches to the
// correct Format method based on the concrete type. It handles the case where
// the expression is an interface type.
func formatExpr(expr interfaces.Expr, depth int) string {
	type formattable interface {
		Format(depth int) string
	}
	if f, ok := expr.(formattable); ok {
		return f.Format(depth)
	}
	return expr.String() // fallback
}

// formatExprWithParens formats a sub-expression of a binary operator, adding
// parentheses if needed based on operator precedence.
func formatExprWithParens(expr interfaces.Expr, parentOp string) string {
	// Check if the child expression is itself a binary operator with lower
	// or equal precedence.
	if call, ok := expr.(*ExprCall); ok && call.Name == operators.OperatorFuncName {
		if len(call.Args) == 3 {
			if opExpr, ok := call.Args[0].(*ExprStr); ok {
				childPrec := operatorPrecedence(opExpr.V)
				parentPrec := operatorPrecedence(parentOp)
				if childPrec > 0 && parentPrec > 0 && childPrec < parentPrec {
					return "(" + call.Format(0) + ")"
				}
			}
		}
	}
	return formatExpr(expr, 0)
}

// formatArgs formats a list of function/class arguments for declarations.
func formatArgs(args []*interfaces.Arg) string {
	parts := []string{}
	for _, a := range args {
		s := "$" + a.Name
		if a.Type != nil {
			s += " " + a.Type.String()
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// formatCallArgs formats a list of expressions as comma-separated call args.
func formatCallArgs(args []interfaces.Expr) string {
	parts := []string{}
	for _, a := range args {
		parts = append(parts, formatExpr(a, 0))
	}
	return strings.Join(parts, ", ")
}

// Format returns the canonically formatted MCL source for this program.
func (obj *StmtProg) Format(depth int) string {
	lines := []string{}
	prevEndLine := -1
	for _, stmt := range obj.Body {
		// Preserve a single blank line between statements when the
		// original source had a gap of 2+ lines. Multiple blank lines
		// are collapsed into one.
		if pn, ok := stmt.(interfaces.PositionableNode); ok && pn.IsSet() && prevEndLine >= 0 {
			row, _ := pn.Pos()
			if row-prevEndLine > 1 {
				lines = append(lines, "")
			}
		}

		type formattable interface {
			Format(depth int) string
		}
		if f, ok := stmt.(formattable); ok {
			lines = append(lines, f.Format(depth))
		}

		if pn, ok := stmt.(interfaces.PositionableNode); ok && pn.IsSet() {
			prevEndLine, _ = pn.End()
		}
	}
	return strings.Join(lines, "\n")
}

// Format returns the canonically formatted MCL source for this binding.
func (obj *StmtBind) Format(depth int) string {
	prefix := fmtIndent(depth) + "$" + obj.Ident
	if obj.Type != nil {
		prefix += " " + obj.Type.String()
	}
	return prefix + " = " + formatExpr(obj.Value, depth)
}

// resContentSection classifies a resource content element into one of three
// sections: 0=field, 1=meta, 2=edge. Comments return -1 (no section change).
func resContentSection(c StmtResContents) int {
	switch c.(type) {
	case *StmtResMeta:
		return 1
	case *StmtResEdge:
		return 2
	case *StmtResComment:
		return -1 // comments don't trigger section changes
	default:
		return 0 // StmtResField and anything else
	}
}

// Format returns the canonically formatted MCL source for this resource.
func (obj *StmtRes) Format(depth int) string {
	ind := fmtIndent(depth)
	prefix := ""
	if obj.Collect {
		prefix = "collect "
	}
	s := ind + prefix + obj.Kind + " " + formatExpr(obj.Name, depth) + " {"
	if len(obj.Contents) == 0 {
		return s + "}"
	}
	s += "\n"
	lastSection := -1
	for _, c := range obj.Contents {
		// Insert blank line between field/meta/edge sections.
		sec := resContentSection(c)
		if sec >= 0 && lastSection >= 0 && sec != lastSection {
			s += "\n"
		}
		if sec >= 0 {
			lastSection = sec
		}

		type formattable interface {
			Format(depth int) string
		}
		if f, ok := c.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
	}
	s += ind + "}"
	return s
}

// Format returns the canonically formatted MCL source for this comment.
func (obj *StmtComment) Format(depth int) string {
	// The lexer strips the leading # but preserves any space after it, so
	// obj.Value may start with a space already (e.g., " hello"). We just
	// re-prepend the # character.
	return fmtIndent(depth) + "#" + obj.Value
}

// Format returns the canonically formatted MCL source for this if statement.
func (obj *StmtIf) Format(depth int) string {
	ind := fmtIndent(depth)

	// Detect the panic desugaring pattern: StmtIf wrapping a _panic resource.
	// The parser creates: StmtIf{Condition: ExprCall{panic(...)}, ThenBranch: StmtRes{Kind: "_panic"}}
	if obj.ElseBranch == nil && obj.ThenBranch != nil {
		if res, ok := obj.ThenBranch.(*StmtRes); ok && res.Kind == interfaces.PanicResKind {
			if call, ok := obj.Condition.(*ExprCall); ok {
				name := call.Name
				if name == funcs.PanicFuncName || name == funcs.PanicDebugFuncName {
					return ind + "panic(" + formatCallArgs(call.Args) + ")"
				}
			}
		}
	}

	s := ind + "if " + formatExpr(obj.Condition, depth) + " {\n"
	if obj.ThenBranch != nil {
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := obj.ThenBranch.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
	}
	if obj.ElseBranch != nil {
		s += ind + "} else {\n"
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := obj.ElseBranch.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
		s += ind + "}"
	} else {
		s += ind + "}"
	}
	return s
}

// Format returns the canonically formatted MCL source for this for loop.
func (obj *StmtFor) Format(depth int) string {
	ind := fmtIndent(depth)
	s := ind + "for $" + obj.Index + ", $" + obj.Value + " in " + formatExpr(obj.Expr, depth) + " {\n"
	if obj.Body != nil {
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := obj.Body.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
	}
	s += ind + "}"
	return s
}

// Format returns the canonically formatted MCL source for this forkv loop.
func (obj *StmtForKV) Format(depth int) string {
	ind := fmtIndent(depth)
	s := ind + "forkv $" + obj.Key + ", $" + obj.Val + " in " + formatExpr(obj.Expr, depth) + " {\n"
	if obj.Body != nil {
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := obj.Body.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
	}
	s += ind + "}"
	return s
}

// Format returns the canonically formatted MCL source for this named function.
func (obj *StmtFunc) Format(depth int) string {
	ind := fmtIndent(depth)
	// The Func field is an ExprFunc.
	fn, ok := obj.Func.(*ExprFunc)
	if !ok {
		return ind + "func " + obj.Name + "() { ??? }"
	}
	s := ind + "func " + obj.Name + "(" + formatArgs(fn.Args) + ")"
	if fn.Return != nil {
		s += " " + fn.Return.String()
	}
	s += " {\n"
	if fn.Body != nil {
		s += fmtIndent(depth+1) + formatExpr(fn.Body, depth+1) + "\n"
	}
	s += ind + "}"
	return s
}

// Format returns the canonically formatted MCL source for this class.
func (obj *StmtClass) Format(depth int) string {
	ind := fmtIndent(depth)
	s := ind + "class " + obj.Name
	if obj.Args != nil {
		s += "(" + formatArgs(obj.Args) + ")"
	}
	s += " {\n"
	if obj.Body != nil {
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := obj.Body.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}
	}
	s += ind + "}"
	return s
}

// Format returns the canonically formatted MCL source for this include.
func (obj *StmtInclude) Format(depth int) string {
	s := fmtIndent(depth) + "include " + obj.Name
	if obj.Args != nil {
		s += "(" + formatCallArgs(obj.Args) + ")"
	}
	if obj.Alias != "" {
		s += " as " + obj.Alias
	}
	return s
}

// Format returns the canonically formatted MCL source for this import.
func (obj *StmtImport) Format(depth int) string {
	s := fmtIndent(depth) + "import " + strconv.Quote(obj.Name)
	if obj.Alias != "" {
		if obj.Alias == "*" {
			s += " as *"
		} else {
			s += " as " + obj.Alias
		}
	}
	return s
}

// Format returns the canonically formatted MCL source for this edge statement.
func (obj *StmtEdge) Format(depth int) string {
	parts := []string{}
	for _, eh := range obj.EdgeHalfList {
		parts = append(parts, eh.Format(0))
	}
	sep := " -> "
	if obj.Notify {
		sep = " -> " // notify edges still use -> in syntax
	}
	return fmtIndent(depth) + strings.Join(parts, sep)
}

// Format returns the canonically formatted MCL source for this edge half.
func (obj *StmtEdgeHalf) Format(depth int) string {
	// Kind is stored lowercase in the AST, but edges use CamelCase.
	// Handle colon-separated kinds like "aws:ec2" -> "Aws:Ec2".
	segments := strings.Split(obj.Kind, ":")
	for i, seg := range segments {
		segments[i] = strings.Title(seg)
	}
	kind := strings.Join(segments, ":")
	s := kind + "[" + formatExpr(obj.Name, 0) + "]"
	if obj.SendRecv != "" {
		s += "." + obj.SendRecv
	}
	return s
}

// Format returns the canonically formatted MCL source for this resource field.
func (obj *StmtResField) Format(depth int) string {
	ind := fmtIndent(depth)
	if obj.Condition != nil {
		return ind + obj.Field + " => " + formatExpr(obj.Condition, 0) + " ?: " + formatExpr(obj.Value, 0) + ","
	}
	return ind + obj.Field + " => " + formatExpr(obj.Value, 0) + ","
}

// Format returns the canonically formatted MCL source for this resource edge.
func (obj *StmtResEdge) Format(depth int) string {
	ind := fmtIndent(depth)
	// Property is stored lowercase by the lexer but written capitalized.
	prop := strings.Title(obj.Property)
	ehStr := obj.EdgeHalf.Format(0)
	if obj.Condition != nil {
		return ind + prop + " => " + formatExpr(obj.Condition, 0) + " ?: " + ehStr + ","
	}
	return ind + prop + " => " + ehStr + ","
}

// Format returns the canonically formatted MCL source for this resource meta.
func (obj *StmtResMeta) Format(depth int) string {
	ind := fmtIndent(depth)
	if obj.Condition != nil {
		return ind + "Meta:" + obj.Property + " => " + formatExpr(obj.Condition, 0) + " ?: " + formatExpr(obj.MetaExpr, 0) + ","
	}
	return ind + "Meta:" + obj.Property + " => " + formatExpr(obj.MetaExpr, 0) + ","
}

// Format returns the canonically formatted MCL source for this resource comment.
func (obj *StmtResComment) Format(depth int) string {
	return fmtIndent(depth) + "#" + obj.Value
}

// Format returns the canonically formatted MCL source for this boolean.
func (obj *ExprBool) Format(depth int) string {
	return strconv.FormatBool(obj.V)
}

// Format returns the canonically formatted MCL source for this string.
// The lexer stores string values with escape sequences preserved (e.g., \n is
// stored as literal backslash-n, not a newline character), so we just re-wrap
// the value in double quotes.
func (obj *ExprStr) Format(depth int) string {
	return "\"" + obj.V + "\""
}

// Format returns the canonically formatted MCL source for this integer.
func (obj *ExprInt) Format(depth int) string {
	return strconv.FormatInt(obj.V, 10)
}

// Format returns the canonically formatted MCL source for this float.
func (obj *ExprFloat) Format(depth int) string {
	s := strconv.FormatFloat(obj.V, 'f', -1, 64)
	// Ensure there is always a decimal point so it parses as float.
	if !strings.Contains(s, ".") {
		s += ".0"
	}
	return s
}

// Format returns the canonically formatted MCL source for this list.
func (obj *ExprList) Format(depth int) string {
	if len(obj.Elements) == 0 {
		return "[]"
	}
	parts := []string{}
	for _, e := range obj.Elements {
		parts = append(parts, formatExpr(e, 0) + ",")
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// Format returns the canonically formatted MCL source for this map.
func (obj *ExprMap) Format(depth int) string {
	if len(obj.KVs) == 0 {
		return "{}"
	}
	parts := []string{}
	for _, kv := range obj.KVs {
		parts = append(parts, formatExpr(kv.Key, 0)+" => "+formatExpr(kv.Val, 0)+",")
	}
	return "{" + strings.Join(parts, " ") + "}"
}

// Format returns the canonically formatted MCL source for this struct.
func (obj *ExprStruct) Format(depth int) string {
	if len(obj.Fields) == 0 {
		return "struct{}"
	}
	parts := []string{}
	for _, f := range obj.Fields {
		parts = append(parts, f.Name+" => "+formatExpr(f.Value, 0)+",")
	}
	return "struct{" + strings.Join(parts, " ") + "}"
}

// Format returns the canonically formatted MCL source for this variable.
func (obj *ExprVar) Format(depth int) string {
	return "$" + obj.Name
}

// Format returns the canonically formatted MCL source for this lambda.
func (obj *ExprFunc) Format(depth int) string {
	// Built-in functions that don't have a Body can't be formatted.
	if obj.Body == nil {
		if obj.Title != "" {
			return obj.Title
		}
		return "<built-in>"
	}
	s := "func(" + formatArgs(obj.Args) + ")"
	if obj.Return != nil {
		s += " " + obj.Return.String()
	}
	s += " { " + formatExpr(obj.Body, 0) + " }"
	return s
}

// Format returns the canonically formatted MCL source for this if expression.
func (obj *ExprIf) Format(depth int) string {
	s := "if " + formatExpr(obj.Condition, 0)
	s += " { " + formatExpr(obj.ThenBranch, 0) + " }"
	s += " else"
	s += " { " + formatExpr(obj.ElseBranch, 0) + " }"
	return s
}

// Format returns the canonically formatted MCL source for this function call.
// This is the most complex formatter because the parser desugars many syntactic
// constructs (operators, lookups, struct access, etc.) into ExprCall nodes.
func (obj *ExprCall) Format(depth int) string {
	// Binary and unary operators: _operator("op", left, right) or _operator("op", expr)
	if obj.Name == operators.OperatorFuncName {
		if len(obj.Args) >= 2 {
			if opExpr, ok := obj.Args[0].(*ExprStr); ok {
				op := opExpr.V
				if len(obj.Args) == 2 {
					// Unary operator (not).
					return op + " " + formatExprWithParens(obj.Args[1], op)
				}
				if len(obj.Args) == 3 {
					// Binary operator.
					left := formatExprWithParens(obj.Args[1], op)
					right := formatExprWithParens(obj.Args[2], op)
					return left + " " + op + " " + right
				}
			}
		}
	}

	// Index lookup: _lookup(container, key)
	if obj.Name == funcs.LookupFuncName && len(obj.Args) == 2 {
		return formatExpr(obj.Args[0], 0) + "[" + formatExpr(obj.Args[1], 0) + "]"
	}

	// Index lookup with default: _lookup_default(container, key, default)
	if obj.Name == funcs.LookupDefaultFuncName && len(obj.Args) == 3 {
		return formatExpr(obj.Args[0], 0) + "[" + formatExpr(obj.Args[1], 0) + "] || " + formatExpr(obj.Args[2], 0)
	}

	// Struct field lookup: _struct_lookup(struct, "field")
	if obj.Name == funcs.StructLookupFuncName && len(obj.Args) == 2 {
		if fieldExpr, ok := obj.Args[1].(*ExprStr); ok {
			return formatExpr(obj.Args[0], 0) + "->" + fieldExpr.V
		}
	}

	// Struct field lookup with default: _struct_lookup_optional(struct, "field", default)
	if obj.Name == funcs.StructLookupOptionalFuncName && len(obj.Args) == 3 {
		if fieldExpr, ok := obj.Args[1].(*ExprStr); ok {
			return formatExpr(obj.Args[0], 0) + "->" + fieldExpr.V + " || " + formatExpr(obj.Args[2], 0)
		}
	}

	// Membership test: contains(elem, list)
	if obj.Name == funcs.ContainsFuncName && len(obj.Args) == 2 {
		return formatExpr(obj.Args[0], 0) + " in " + formatExpr(obj.Args[1], 0)
	}

	// Lambda call: $name(args...)
	if obj.Var {
		return "$" + obj.Name + "(" + formatCallArgs(obj.Args) + ")"
	}

	// Anonymous function call: func(...)(...)(args...)
	if obj.Anon != nil {
		return "(" + formatExpr(obj.Anon, 0) + ")(" + formatCallArgs(obj.Args) + ")"
	}

	// Regular function call: name(args...)
	return obj.Name + "(" + formatCallArgs(obj.Args) + ")"
}

// FormatNode formats any AST node that implements a Format method. This is a
// convenience function for use by the CLI formatter.
func FormatNode(node interfaces.Node) string {
	type formattable interface {
		Format(depth int) string
	}
	if f, ok := node.(formattable); ok {
		return f.Format(0)
	}
	return fmt.Sprintf("%s", node)
}

// CommentData is the data for a single comment that was collected by the lexer.
// This mirrors parser.Comment but is defined here to avoid import cycles.
type CommentData struct {
	Value  string
	Row    int
	Col    int
	Inline bool
}

// FormatWithComments formats an AST node and interleaves comments from the
// side-channel comment list collected by the lexer. It returns the formatted
// source code.
func FormatWithComments(node interfaces.Node, comments interface{}) string {
	// Format the AST without comments first.
	formatted := FormatNode(node)

	// Convert the comments to our local type. We accept interface{} to
	// avoid import cycles with the parser package.
	var cmts []*CommentData
	if cs, ok := comments.([]*CommentData); ok {
		cmts = cs
	} else {
		// The caller may pass parser.Comment slice — we handle this via
		// reflection-free approach: the caller converts before passing.
		return formatted
	}

	if len(cmts) == 0 {
		return formatted
	}

	// For standalone comments, we insert them based on their original line
	// position relative to AST node positions. Inline comments are appended
	// to the end of their associated line.
	//
	// Strategy: Split formatted output into lines. For each comment, find
	// the right position to insert it. Since the formatter may reflow
	// lines, we use the AST's original source positions to determine where
	// comments belong.
	//
	// For now, use a simple approach: insert standalone comments at the
	// positions determined by their relationship to the AST's StmtProg Body
	// statements using the original line numbers.
	if prog, ok := node.(*StmtProg); ok {
		return formatProgWithComments(prog, cmts, 0)
	}

	return formatted
}

// formatProgWithComments formats a StmtProg and interleaves comments.
func formatProgWithComments(prog *StmtProg, comments []*CommentData, depth int) string {
	ind := fmtIndent(depth)
	lines := []string{}
	commentIdx := 0
	prevEndLine := -1

	for _, stmt := range prog.Body {
		// Get the original start line of this statement.
		stmtStartLine := -1
		if pn, ok := stmt.(interfaces.PositionableNode); ok && pn.IsSet() {
			row, _ := pn.Pos()
			stmtStartLine = row
		}

		// Insert any comments that come before this statement.
		for commentIdx < len(comments) && stmtStartLine >= 0 && comments[commentIdx].Row < stmtStartLine {
			c := comments[commentIdx]
			if !c.Inline {
				// Blank line before this comment if there's a gap.
				if prevEndLine >= 0 && c.Row-prevEndLine > 1 {
					lines = append(lines, "")
				}
				lines = append(lines, ind+"#"+c.Value)
				prevEndLine = c.Row
			}
			commentIdx++
		}

		// Preserve a single blank line when the original source had a
		// gap of 2+ lines. Multiple blank lines collapse into one.
		if stmtStartLine >= 0 && prevEndLine >= 0 && stmtStartLine-prevEndLine > 1 {
			lines = append(lines, "")
		}

		// For resource statements, format with embedded comments.
		if res, ok := stmt.(*StmtRes); ok {
			stmtEndLine := -1
			if pn, ok := stmt.(interfaces.PositionableNode); ok && pn.IsSet() {
				stmtEndLine, _ = pn.End()
			}
			// Collect comments that fall within this resource.
			var resCmts []*CommentData
			for commentIdx < len(comments) && stmtEndLine >= 0 && comments[commentIdx].Row <= stmtEndLine {
				resCmts = append(resCmts, comments[commentIdx])
				commentIdx++
			}
			lines = append(lines, formatResWithComments(res, resCmts, depth))
			prevEndLine = stmtEndLine
			continue
		}

		// Format the statement.
		type formattable interface {
			Format(depth int) string
		}
		if f, ok := stmt.(formattable); ok {
			lines = append(lines, f.Format(depth))
		}

		// Skip past any comments that are on lines covered by this stmt.
		stmtEndLine := -1
		if pn, ok := stmt.(interfaces.PositionableNode); ok && pn.IsSet() {
			stmtEndLine, _ = pn.End()
		}
		for commentIdx < len(comments) && stmtEndLine >= 0 && comments[commentIdx].Row <= stmtEndLine {
			commentIdx++
		}
		prevEndLine = stmtEndLine
	}

	// Append any trailing comments.
	for commentIdx < len(comments) {
		c := comments[commentIdx]
		if !c.Inline {
			lines = append(lines, ind+"#"+c.Value)
		}
		commentIdx++
	}

	return strings.Join(lines, "\n")
}

// formatResWithComments formats a resource statement and interleaves comments
// from the side channel that fall within the resource body.
func formatResWithComments(res *StmtRes, comments []*CommentData, depth int) string {
	ind := fmtIndent(depth)
	prefix := ""
	if res.Collect {
		prefix = "collect "
	}
	s := ind + prefix + res.Kind + " " + formatExpr(res.Name, depth) + " {"
	if len(res.Contents) == 0 && len(comments) == 0 {
		return s + "}"
	}
	s += "\n"

	commentIdx := 0
	lastSection := -1
	for _, c := range res.Contents {
		// Get the original start line of this content element.
		contentStartLine := -1
		if pn, ok := c.(interfaces.PositionableNode); ok && pn.IsSet() {
			row, _ := pn.Pos()
			contentStartLine = row
		}

		// Insert blank line between field/meta/edge sections.
		sec := resContentSection(c)
		if sec >= 0 && lastSection >= 0 && sec != lastSection {
			s += "\n"
		}

		// Insert comments that come before this content element.
		for commentIdx < len(comments) && contentStartLine >= 0 && comments[commentIdx].Row < contentStartLine {
			cm := comments[commentIdx]
			if !cm.Inline {
				s += fmtIndent(depth+1) + "#" + cm.Value + "\n"
			}
			commentIdx++
		}

		if sec >= 0 {
			lastSection = sec
		}

		type formattable interface {
			Format(depth int) string
		}
		if f, ok := c.(formattable); ok {
			s += f.Format(depth+1) + "\n"
		}

		// Skip past comments covered by this content element.
		contentEndLine := -1
		if pn, ok := c.(interfaces.PositionableNode); ok && pn.IsSet() {
			contentEndLine, _ = pn.End()
		}
		for commentIdx < len(comments) && contentEndLine >= 0 && comments[commentIdx].Row <= contentEndLine {
			commentIdx++
		}
	}

	// Trailing comments inside the resource body.
	for commentIdx < len(comments) {
		cm := comments[commentIdx]
		if !cm.Inline {
			s += fmtIndent(depth+1) + "#" + cm.Value + "\n"
		}
		commentIdx++
	}

	s += ind + "}"
	return s
}
