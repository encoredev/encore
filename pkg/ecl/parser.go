package ecl

import (
	"fmt"
	"strings"
)

const maxParseErrors = 20

// ParseFile parses a single ECL source file. The filename is used in
// positions and diagnostics only; no file I/O is performed.
//
// On error it returns the partial file parsed so far together with an
// ErrorList describing every problem found.
func ParseFile(filename string, src []byte) (*File, error) {
	var diags ErrorList
	sf := newSourceFile(filename, string(src))
	lx := newLexer(sf, &diags)
	p := &parser{
		file:  &File{Path: filename, src: sf},
		src:   sf,
		toks:  lx.lex(),
		diags: &diags,
	}
	p.parseFile()
	for _, r := range p.file.Rules {
		r.file = p.file
	}
	diags.sort()
	return p.file, diags.Err()
}

type parser struct {
	file  *File
	src   *sourceFile
	toks  []token
	i     int
	diags *ErrorList
}

// --- token helpers ---

func (p *parser) cur() token          { return p.toks[p.i] }
func (p *parser) at(k tokenKind) bool { return p.cur().kind == k }

func (p *parser) advance() token {
	t := p.toks[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

func (p *parser) accept(k tokenKind) (token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	return p.cur(), false
}

// expect consumes a token of the given kind, or reports an error like
// "expected '{' to begin the rule body, found newline".
func (p *parser) expect(k tokenKind, context string) (token, bool) {
	if p.at(k) {
		return p.advance(), true
	}
	t := p.cur()
	p.errorAtToken(t, "expected %s %s, found %s", k, context, t.describe())
	return t, false
}

func (p *parser) skipNewlines() {
	for p.at(tokNewline) {
		p.advance()
	}
}

// expectTerminator requires the current statement to end here: a newline
// (consumed), or '}' / end of file (left in place).
func (p *parser) expectTerminator(context string) bool {
	switch p.cur().kind {
	case tokNewline:
		p.advance()
		return true
	case tokRBrace, tokEOF:
		return true
	default:
		t := p.cur()
		p.errorAtToken(t, "expected newline after %s, found %s", context, t.describe())
		return false
	}
}

func (p *parser) tooManyErrors() bool {
	if len(*p.diags) < maxParseErrors {
		return false
	}
	last := (*p.diags)[len(*p.diags)-1]
	if !strings.HasPrefix(last.Message, "too many errors") {
		p.errorAt(p.cur().pos, p.cur().end, "too many errors; stopping")
	}
	return true
}

// --- error helpers ---

func (p *parser) errorAt(start, end Position, format string, args ...any) *Diagnostic {
	return p.diags.addf(p.src, start, end, format, args...)
}

func (p *parser) errorAtToken(t token, format string, args ...any) *Diagnostic {
	return p.errorAt(t.pos, t.end, format, args...)
}

// syncLine skips tokens until just past the next newline, or until '}'
// or end of file (left in place).
func (p *parser) syncLine() {
	for {
		switch p.cur().kind {
		case tokNewline:
			p.advance()
			return
		case tokRBrace, tokEOF:
			return
		}
		p.advance()
	}
}

// syncTopLevel skips tokens until the next declaration keyword ('for' or
// 'import'), a '}' (which may close an enclosing if block), or end of
// file. It also stops at an identifier that begins a named/dynamic block.
func (p *parser) syncTopLevel() {
	for {
		switch p.cur().kind {
		case tokFor, tokImport, tokWhere, tokIdent, tokRBrace, tokEOF:
			return
		}
		p.advance()
	}
}

// syncDecl recovers from an unexpected token in declaration position:
// it always consumes at least one token, then skips to the next
// declaration start, '}', or end of file.
func (p *parser) syncDecl() {
	p.advance()
	for {
		switch p.cur().kind {
		case tokFor, tokWhere, tokImport, tokIdent, tokRBrace, tokEOF:
			return
		}
		p.advance()
	}
}

// --- grammar ---

func (p *parser) parseFile() {
	p.skipNewlines()
	if p.atVersionDecl() {
		p.parseVersion()
		p.skipNewlines()
	}
	for p.at(tokImport) {
		p.parseImport()
		p.skipNewlines()
	}
	for !p.at(tokEOF) && !p.tooManyErrors() {
		p.parseDecl(nil, true)
		p.skipNewlines()
	}
}

// parseDecl parses one declaration: a 'for' rule, a named or dynamic
// resource block, or an 'if' block. outer holds the selector conditions
// of enclosing if blocks, which are prepended to each rule's own
// selector.
func (p *parser) parseDecl(outer []*Condition, topLevel bool) {
	// A `version <number>` here is misplaced: the legitimate version
	// declaration is parsed in parseFile before any declaration.
	if p.atVersionDecl() {
		d := p.errorAtToken(p.cur(), "the version declaration must be the first statement in the file")
		d.Hint = "move 'version' to the top of the file"
		p.syncLine()
		return
	}
	switch p.cur().kind {
	case tokFor:
		if r := p.parseForRule(outer); r != nil {
			p.file.Rules = append(p.file.Rules, r)
		}
	case tokIdent:
		if r := p.parseResourceBlock(outer); r != nil {
			p.file.Rules = append(p.file.Rules, r)
		}
	case tokIf:
		p.parseIfBlock(outer)
	case tokWhere:
		// Migration aid: the standalone `if` block is now `if`.
		d := p.errorAtToken(p.cur(), "'where' blocks are now written as 'if' blocks")
		d.Hint = `scope rules to an environment with 'if', e.g.: if env.type == "production" { ... }`
		p.parseIfBlock(outer) // recover by parsing it as an if block
	case tokImport:
		p.errorAtToken(p.cur(), "import declarations must appear before the first rule")
		p.parseImport()
	default:
		t := p.cur()
		if topLevel {
			p.errorAtToken(t, "expected 'for', a resource block, or 'if' to begin a declaration, found %s", t.describe())
		} else {
			p.errorAtToken(t, "expected 'for', a resource block, or a nested 'if' block, found %s", t.describe())
		}
		p.syncDecl()
	}
}

// parseIfBlock parses `if <condition> { <decl>* }` and desugars it: the block's
// conditions are prepended to every rule inside, composing with nested blocks
// via &&. The conditions are marked EnvScoped, since an `if` block is evaluated
// in the top-level environment scope; Validate checks they reference only
// declared environment attributes.
func (p *parser) parseIfBlock(outer []*Condition) {
	p.advance() // 'if' (or 'where', when recovering)
	var conds []*Condition
	if p.at(tokLBrace) {
		d := p.errorAtToken(p.cur(), "expected a condition after 'if'")
		d.Hint = `e.g.: if env.type == "production" { ... }`
	} else {
		conds = p.parseSelector()
	}
	for _, c := range conds {
		c.EnvScoped = true
	}
	combined := combineConds(outer, conds)

	if _, ok := p.expect(tokLBrace, "to begin the if block"); !ok {
		p.syncTopLevel()
		return
	}
	p.skipNewlines()
	for !p.at(tokRBrace) && !p.at(tokEOF) && !p.tooManyErrors() {
		p.parseDecl(combined, false)
		p.skipNewlines()
	}
	p.expect(tokRBrace, "to close the if block")
	p.expectTerminator("the if block")
}

// combineConds concatenates enclosing if-block conditions with a
// rule's own, copying so rules never share backing arrays.
func combineConds(outer, own []*Condition) []*Condition {
	if len(outer) == 0 {
		return own
	}
	conds := make([]*Condition, 0, len(outer)+len(own))
	conds = append(conds, outer...)
	return append(conds, own...)
}

// atVersionDecl reports whether the parser is at a `version <number>`
// declaration. "version" is not a keyword, so this is contextual.
func (p *parser) atVersionDecl() bool {
	return p.at(tokIdent) && p.cur().str == "version" && p.toks[p.i+1].kind == tokNumber
}

func (p *parser) parseVersion() {
	p.advance() // 'version'
	t, ok := p.expect(tokNumber, "after 'version'")
	if !ok {
		p.syncLine()
		return
	}
	if t.unit != "" || t.num != float64(int(t.num)) {
		p.errorAtToken(t, "version must be an integer, found '%s'", t.text)
	} else if int(t.num) != 1 {
		d := p.errorAtToken(t, "unsupported language version %d", int(t.num))
		d.Hint = "this parser supports version 1"
	}
	p.file.Version = &Version{Pos: t.pos, Num: int(t.num)}
	p.expectTerminator("the version declaration")
}

func (p *parser) parseImport() {
	kw := p.advance() // 'import'
	t, ok := p.expect(tokString, "with the file path after 'import'")
	if !ok {
		p.syncLine()
		return
	}
	if t.str == "" {
		p.errorAtToken(t, "import path must not be empty")
	}
	p.file.Imports = append(p.file.Imports, &Import{
		Pos:     kw.pos,
		Path:    t.str,
		PathPos: t.pos,
		PathEnd: t.end,
	})
	p.expectTerminator("the import declaration")
}

// parseForRule parses `for <kind> [if <selector>] { ... }`, a block
// configuring all resources of a kind matching the selector.
func (p *parser) parseForRule(outer []*Condition) *Rule {
	kw := p.advance() // 'for'
	rule := &Rule{Pos: kw.pos}

	kindTok := p.cur()
	switch kindTok.kind {
	case tokIdent:
		p.advance()
		rule.Kind, rule.KindPos, rule.KindEnd = kindTok.str, kindTok.pos, kindTok.end
	case tokString:
		d := p.errorAtToken(kindTok, "expected a resource kind after 'for', found %s", kindTok.describe())
		d.Hint = fmt.Sprintf("the resource kind comes before the name: %s { ... } configures all of a kind", kindTok.text)
		p.syncTopLevel()
		return nil
	default:
		d := p.errorAtToken(kindTok, "expected a resource kind after 'for', found %s", kindTok.describe())
		d.Hint = "e.g.: for service { ... }"
		p.syncTopLevel()
		return nil
	}

	// `for` blocks take no name or expression — only a selector.
	switch p.cur().kind {
	case tokString:
		t := p.cur()
		d := p.errorAtToken(t, "a named block omits 'for'")
		d.Hint = fmt.Sprintf("write: %s %s { ... } to configure one resource", rule.Kind, t.text)
		p.syncTopLevel()
		return nil
	case tokIdent:
		t := p.cur()
		d := p.errorAtToken(t, "a dynamic block omits 'for'")
		d.Hint = fmt.Sprintf("write: %s <expr> { ... }", rule.Kind)
		p.syncTopLevel()
		return nil
	}

	p.parseRuleHeaderTail(rule, outer)
	return rule
}

// parseResourceBlock parses a named (`<kind> "name"`) or dynamic
// (`<kind> <expr>`) resource block, optionally with an `if` selector.
func (p *parser) parseResourceBlock(outer []*Condition) *Rule {
	kindTok := p.advance() // kind identifier
	rule := &Rule{Pos: kindTok.pos, Kind: kindTok.str, KindPos: kindTok.pos, KindEnd: kindTok.end}

	// Migration aid: 'define' is no longer a keyword.
	if kindTok.str == "define" && (p.at(tokIdent) || p.at(tokString)) {
		d := p.errorAtToken(kindTok, "the 'define' keyword has been removed")
		d.Hint = "declare managed resources directly, e.g.: sql_cluster \"main\" { ... }"
		p.syncTopLevel()
		return nil
	}

	switch p.cur().kind {
	case tokString:
		t := p.advance()
		rule.Name, rule.NamePos = t.str, t.pos
		if t.str == "" {
			p.errorAtToken(t, "resource name must not be empty")
		}
	case tokIdent:
		expr, span, ok := p.parseFieldPath("dynamic block")
		if !ok {
			p.syncTopLevel()
			return rule
		}
		rule.DynExpr, rule.DynExprPos, rule.DynExprEnd = expr, span.Start, span.End
	case tokWhere:
		d := p.errorAtToken(p.cur(), "to match resources of kind '%s' by selector, use 'for'", rule.Kind)
		d.Hint = fmt.Sprintf("write: for %s if ... { ... }", rule.Kind)
		p.syncTopLevel()
		return rule
	case tokLBrace:
		d := p.errorAtToken(p.cur(), "a resource block needs a name or expression")
		d.Hint = fmt.Sprintf("write '%s \"name\" { ... }' for one resource, or 'for %s { ... }' for all", rule.Kind, rule.Kind)
		p.syncTopLevel()
		return rule
	case tokColon, tokAssign:
		d := p.errorAtToken(p.cur(), "property rules must appear inside a rule body, not at this level")
		d.Hint = fmt.Sprintf("wrap it in a block, e.g.: for %s { %s: ... }", rule.Kind, rule.Kind)
		p.syncLine()
		return nil
	default:
		t := p.cur()
		p.errorAtToken(t, "expected a resource name or expression after '%s', found %s", rule.Kind, t.describe())
		p.syncTopLevel()
		return rule
	}

	p.parseRuleHeaderTail(rule, outer)
	return rule
}

// parseRuleHeaderTail parses an optional `if` selector followed by the
// rule body.
func (p *parser) parseRuleHeaderTail(rule *Rule, outer []*Condition) {
	var own []*Condition
	if _, ok := p.accept(tokIf); ok {
		own = p.parseSelector()
	} else if p.at(tokWhere) {
		// Migration aid: rule conditions now use 'if', not 'where'.
		d := p.errorAtToken(p.cur(), "conditions on a rule now use 'if', not 'where'")
		d.Hint = `e.g.: for service if env.type == "production" { ... }`
		p.advance()
		own = p.parseSelector()
	}
	rule.Where = combineConds(outer, own)

	if !p.at(tokLBrace) {
		t := p.cur()
		d := p.errorAtToken(t, "expected '{' to begin the rule body, found %s", t.describe())
		if t.kind == tokIdent && len(own) > 0 {
			d.Hint = "selector conditions are combined with '&&'"
		}
		p.syncTopLevel()
		return
	}
	p.parseRuleBody(rule)
}

// parseRuleBody parses `{ (property | nested-block)* }`.
func (p *parser) parseRuleBody(rule *Rule) {
	p.advance() // '{'
	p.skipNewlines()
	for !p.at(tokRBrace) && !p.at(tokEOF) && !p.tooManyErrors() {
		// Migration aid: 'require' blocks have been removed.
		if p.at(tokIdent) && p.cur().str == "require" && p.toks[p.i+1].kind == tokIdent {
			d := p.errorAtToken(p.cur(), "the 'require' block has been removed")
			d.Hint = "constrain a referenced resource with nested object syntax, e.g.: cluster: { backup_retention: >= 30d }"
			p.syncLine()
			p.skipNewlines()
			continue
		}
		if p.atNestedBlock() {
			if b := p.parseResourceBlock(nil); b != nil {
				rule.Blocks = append(rule.Blocks, b)
			}
		} else if prop := p.parseProperty(); prop != nil {
			rule.Props = append(rule.Props, prop)
		}
		p.skipNewlines()
	}
	p.expect(tokRBrace, "to close the rule body")
	p.expectTerminator("the rule")
}

// atNestedBlock reports whether the body statement at the cursor begins a
// nested resource block (`kind "name" { }` or `kind <expr> { }`) rather
// than a property rule (whose path is followed by '.', ':' or '=').
func (p *parser) atNestedBlock() bool {
	if !p.at(tokIdent) {
		return false
	}
	switch p.toks[p.i+1].kind {
	case tokString, tokIdent:
		return true
	default:
		return false
	}
}

// --- selectors ---

func (p *parser) parseSelector() []*Condition {
	var conds []*Condition
	for {
		if c := p.parseCondition(); c != nil {
			conds = append(conds, c)
		} else {
			// Recovery: skip to something that can continue the selector.
			for {
				switch p.cur().kind {
				case tokAndAnd, tokAmp, tokOrOr, tokPipe, tokLBrace, tokNewline, tokEOF:
				default:
					p.advance()
					continue
				}
				break
			}
		}
		switch p.cur().kind {
		case tokAndAnd:
			p.advance()
		case tokAmp:
			d := p.errorAtToken(p.cur(), "selector conditions are combined with '&&', not '&'")
			d.Hint = "'&' combines property constraints; '&&' combines selector conditions"
			p.advance()
		case tokOrOr, tokPipe:
			d := p.errorAtToken(p.cur(), "'%s' is not supported in selectors", p.cur().text)
			d.Hint = "split the rule into separate rules, or use 'in [a, b]' for membership"
			p.advance()
		default:
			return conds
		}
	}
}

func (p *parser) parseCondition() *Condition {
	field, fieldSpan, ok := p.parseFieldPath("selector condition")
	if !ok {
		return nil
	}
	cond := &Condition{Pos: fieldSpan.Start, Field: field, FieldEnd: fieldSpan.End}

	opTok := p.cur()
	switch opTok.kind {
	case tokEq, tokNeq:
		p.advance()
		v, vspan, ok := p.parseValue(fmt.Sprintf("after '%s'", opTok.text))
		if !ok {
			return nil
		}
		cond.Op = CondEq
		if opTok.kind == tokNeq {
			cond.Op = CondNeq
		}
		cond.Values = []Value{v}
		cond.End = vspan.End
	case tokAssign:
		d := p.errorAtToken(opTok, "'=' is not an operator")
		d.Hint = "use '==' for equality comparisons"
		p.advance()
		v, vspan, ok := p.parseValue("after '='")
		if !ok {
			return nil
		}
		cond.Op = CondEq
		cond.Values = []Value{v}
		cond.End = vspan.End
	case tokExists:
		p.advance()
		cond.Op = CondExists
		cond.End = opTok.end
	case tokIn:
		p.advance()
		cond.Op = CondIn
		if _, ok := p.expect(tokLBracket, "after 'in'"); !ok {
			return nil
		}
		for {
			v, _, ok := p.parseValue("in the membership list")
			if !ok {
				return nil
			}
			cond.Values = append(cond.Values, v)
			if _, ok := p.accept(tokComma); !ok {
				break
			}
			if p.at(tokRBracket) { // trailing comma
				break
			}
		}
		end, ok := p.expect(tokRBracket, "to close the membership list")
		if !ok {
			return nil
		}
		cond.End = end.end
	default:
		d := p.errorAtToken(opTok, "expected '==', '!=', 'in', or 'exists' after '%s', found %s", field, opTok.describe())
		if opTok.kind == tokLBrace {
			d.Message = fmt.Sprintf("incomplete selector condition: '%s' needs an operator such as '==' or 'exists'", field)
		}
		return nil
	}
	return cond
}

// parseFieldPath parses a dotted identifier path like "env.type".
func (p *parser) parseFieldPath(context string) (string, Span, bool) {
	first := p.cur()
	if first.kind != tokIdent {
		if first.kind.isKeyword() {
			d := p.errorAtToken(first, "'%s' is a reserved keyword and cannot be used as a field name", first.text)
			d.Hint = "rename the field, or quote the value if you meant a string"
			return "", Span{}, false
		}
		p.errorAtToken(first, "expected a field name in %s, found %s", context, first.describe())
		return "", Span{}, false
	}
	p.advance()
	path := first.str
	end := first.end
	for p.at(tokDot) {
		p.advance()
		seg := p.cur()
		if seg.kind != tokIdent {
			p.errorAtToken(seg, "expected an identifier after '.', found %s", seg.describe())
			return path, Span{Start: first.pos, End: end}, false
		}
		p.advance()
		path += "." + seg.str
		end = seg.end
	}
	return path, Span{Start: first.pos, End: end}, true
}

// --- properties ---

func (p *parser) parseProperty() *Property {
	path, span, ok := p.parseFieldPath("property rule")
	if !ok {
		p.syncLine()
		return nil
	}
	prop := &Property{Pos: span.Start, PathEnd: span.End, Path: path}

	switch p.cur().kind {
	case tokColon:
		p.advance()
	case tokAssign:
		d := p.errorAtToken(p.cur(), "property rules use ':', not '='")
		d.Hint = fmt.Sprintf("write: %s: <constraint>", path)
		p.advance()
	default:
		t := p.cur()
		p.errorAtToken(t, "expected ':' after property path '%s', found %s", path, t.describe())
		p.syncLine()
		return nil
	}

	c, scalarDef, refDef, ok := p.parsePropertyExpr()
	if !ok {
		prop.Value = p.buildPropertyValue(prop, c, scalarDef, refDef)
		p.syncLine()
		return prop
	}

	// 'default' must be the final clause.
	if scalarDef != nil || refDef != nil {
		switch p.cur().kind {
		case tokPipe, tokAmp, tokAndAnd, tokOrOr:
			d := p.errorAtToken(p.cur(), "'default' must be the last clause in a property rule")
			d.Hint = fmt.Sprintf("write constraints first: %s: <constraint> | default <value>", path)
			prop.Value = p.buildPropertyValue(prop, c, scalarDef, refDef)
			p.syncLine()
			return prop
		}
	}

	prop.Value = p.buildPropertyValue(prop, c, scalarDef, refDef)
	if !p.expectTerminator(fmt.Sprintf("the property rule for '%s'", path)) {
		p.syncLine()
	}
	return prop
}

// parsePropertyExpr parses the right-hand side of a property rule: a
// constraint expression with an optional trailing default, or a bare
// default clause. The default is returned as a scalar (*Default) or a
// reference (*RefDefault); at most one is non-nil.
func (p *parser) parsePropertyExpr() (Constraint, *Default, *RefDefault, bool) {
	if p.at(tokDefault) {
		scalar, ref, ok := p.parseDefaultClause()
		return nil, scalar, ref, ok
	}
	return p.parseOrExpr()
}

func (p *parser) parseOrExpr() (Constraint, *Default, *RefDefault, bool) {
	first, ok := p.parseAndExpr()
	if !ok {
		return first, nil, nil, false
	}
	alts := []Constraint{first}
	for {
		switch p.cur().kind {
		case tokPipe:
			p.advance()
		case tokOrOr:
			d := p.errorAtToken(p.cur(), "constraint alternatives are combined with '|', not '||'")
			d.Hint = "'||' is not part of the language; use a single '|'"
			p.advance()
		default:
			return orOf(alts), nil, nil, true
		}
		if p.at(tokDefault) {
			scalar, ref, ok := p.parseDefaultClause()
			return orOf(alts), scalar, ref, ok
		}
		alt, ok := p.parseAndExpr()
		if !ok {
			return orOf(alts), nil, nil, false
		}
		alts = append(alts, alt)
	}
}

func orOf(alts []Constraint) Constraint {
	if len(alts) == 1 {
		return alts[0]
	}
	c := &OrConstraint{Alts: alts}
	return c
}

func (p *parser) parseAndExpr() (Constraint, bool) {
	first, ok := p.parseTerm()
	if !ok {
		return first, false
	}
	terms := []Constraint{first}
	for {
		switch p.cur().kind {
		case tokAmp:
			p.advance()
		case tokAndAnd:
			d := p.errorAtToken(p.cur(), "property constraints are combined with '&', not '&&'")
			d.Hint = "'&&' combines selector conditions; '&' combines property constraints"
			p.advance()
		default:
			if len(terms) == 1 {
				return first, true
			}
			return &AndConstraint{Terms: terms}, true
		}
		term, ok := p.parseTerm()
		if !ok {
			return &AndConstraint{Terms: terms}, false
		}
		terms = append(terms, term)
	}
}

func (p *parser) parseTerm() (Constraint, bool) {
	t := p.cur()
	switch t.kind {
	case tokRequired:
		p.advance()
		return &RequiredConstraint{Pos: t.pos, End: t.end}, true

	case tokGe, tokLe, tokGt, tokLt, tokNeq, tokEq:
		p.advance()
		v, vspan, ok := p.parseValue(fmt.Sprintf("after '%s'", t.text))
		if !ok {
			return nil, false
		}
		op := comparisonOp(t.kind)
		if isOrderingOp(op) && !v.Kind.isOrdered() {
			d := p.errorAt(t.pos, vspan.End,
				"ordering comparison '%s' requires a number, size, or duration, found %s value %s",
				t.text, v.Kind, v)
			if v.Kind == BoolKind || v.Kind == StringKind {
				d.Hint = fmt.Sprintf("use '==' or '!=' to compare %s values", v.Kind)
			}
		}
		return &Comparison{Pos: t.pos, End: vspan.End, Op: op, Value: v}, true

	case tokAssign:
		d := p.errorAtToken(t, "'=' is not an operator")
		d.Hint = "use '==' for an exact match, or omit the operator entirely"
		p.advance()
		v, vspan, ok := p.parseValue("after '='")
		if !ok {
			return nil, false
		}
		return &Comparison{Pos: t.pos, End: vspan.End, Op: OpEq, Value: v}, true

	case tokDefault:
		// e.g. `cpu: >= 1 & default 2` — wrong combinator before default.
		d := p.errorAtToken(t, "'default' must be separated from constraints with '|'")
		d.Hint = "write: <constraint> | default <value>"
		return nil, false

	case tokLBrace:
		return p.parseObjectConstraint()

	case tokIdent:
		if p.atReference() {
			ref, ok := p.parseReference()
			if !ok {
				return nil, false
			}
			return ref, true
		}
		// A bare identifier is not a value; string values must be quoted.
		p.advance()
		d := p.errorAtToken(t, "string values must be quoted")
		d.Hint = fmt.Sprintf("write %q", t.str)
		return &Comparison{Pos: t.pos, End: t.end, Op: OpEq, Value: String(t.str), Implicit: true}, true

	default:
		v, vspan, ok := p.parseValue("as a constraint")
		if !ok {
			return nil, false
		}
		return &Comparison{Pos: vspan.Start, End: vspan.End, Op: OpEq, Value: v, Implicit: true}, true
	}
}

func comparisonOp(k tokenKind) CompareOp {
	switch k {
	case tokEq:
		return OpEq
	case tokNeq:
		return OpNeq
	case tokGe:
		return OpGe
	case tokLe:
		return OpLe
	case tokGt:
		return OpGt
	case tokLt:
		return OpLt
	}
	panic("not a comparison token")
}

func isOrderingOp(op CompareOp) bool {
	return op == OpGe || op == OpLe || op == OpGt || op == OpLt
}

// parseDefaultClause parses `default <value-or-reference>`, returning a
// scalar (*Default) or reference (*RefDefault) default.
func (p *parser) parseDefaultClause() (*Default, *RefDefault, bool) {
	kw := p.advance() // 'default'
	if p.atReference() {
		ref, ok := p.parseReference()
		if !ok {
			return nil, nil, false
		}
		return nil, &RefDefault{Pos: kw.pos, Ref: ref}, true
	}
	v, vspan, ok := p.parseValue("after 'default'")
	if !ok {
		return nil, nil, false
	}
	return &Default{Pos: kw.pos, Value: v, ValuePos: vspan.Start, ValueEnd: vspan.End}, nil, true
}

// atReference reports whether the cursor is at a reference value
// (`kind.name` or `kind[expr]`).
func (p *parser) atReference() bool {
	return p.at(tokIdent) &&
		(p.toks[p.i+1].kind == tokDot || p.toks[p.i+1].kind == tokLBracket)
}

// parseReference parses a static (`kind.name`) or dynamic (`kind[expr]`)
// reference.
func (p *parser) parseReference() (*Reference, bool) {
	kindTok := p.advance() // kind identifier
	ref := &Reference{Kind: kindTok.str, Pos: kindTok.pos, End: kindTok.end, KindPos: kindTok.pos, KindEnd: kindTok.end}
	switch p.cur().kind {
	case tokDot:
		p.advance()
		name := p.cur()
		switch name.kind {
		case tokIdent, tokString:
			p.advance()
			ref.Mode = StaticRef
			ref.Name = name.str
			ref.End = name.end
			if name.str == "" {
				p.errorAtToken(name, "reference name must not be empty")
			}
			return ref, true
		default:
			p.errorAtToken(name, "expected a resource name after '%s.', found %s", kindTok.str, name.describe())
			return nil, false
		}
	case tokLBracket:
		p.advance()
		expr, _, ok := p.parseFieldPath("dynamic reference")
		if !ok {
			return nil, false
		}
		end, ok := p.expect(tokRBracket, "to close the dynamic reference")
		if !ok {
			return nil, false
		}
		ref.Mode = DynamicRef
		ref.Expr = expr
		ref.End = end.end
		return ref, true
	default:
		p.errorAtToken(p.cur(), "expected '.' or '[' in reference to '%s'", kindTok.str)
		return nil, false
	}
}

// parseObjectConstraint parses a `{ <property>* }` block constraining a
// reference target. Defaults are not allowed inside it.
func (p *parser) parseObjectConstraint() (*ObjectConstraint, bool) {
	lb := p.advance() // '{'
	obj := &ObjectConstraint{Pos: lb.pos, End: lb.end}
	p.skipNewlines()
	for !p.at(tokRBrace) && !p.at(tokEOF) && !p.tooManyErrors() {
		if prop := p.parseProperty(); prop != nil {
			p.rejectObjectDefault(prop)
			obj.Props = append(obj.Props, prop)
		}
		p.skipNewlines()
	}
	if rb, ok := p.expect(tokRBrace, "to close the object constraint"); ok {
		obj.End = rb.end
	}
	return obj, true
}

func (p *parser) rejectObjectDefault(prop *Property) {
	switch v := prop.Value.(type) {
	case *ScalarValue:
		if v.Default != nil {
			d := p.errorAt(v.Default.Pos, v.Default.ValueEnd, "'default' is not allowed inside an object constraint")
			d.Hint = "an object constraint only constrains the referenced resource; set defaults in rules for its own kind"
			v.Default = nil
		}
	case *RefValue:
		if v.Default != nil {
			d := p.errorAt(v.Default.Pos, v.Default.Ref.End, "'default' is not allowed inside an object constraint")
			d.Hint = "an object constraint only constrains the referenced resource"
			v.Default = nil
		}
	}
}

// buildPropertyValue classifies a parsed constraint expression plus default
// clauses into a scalar or reference property value, reporting errors for
// invalid mixes (a reference combined with scalar constraints, two
// references, a reference inside a '|' alternative, or a scalar default on
// a reference property).
func (p *parser) buildPropertyValue(prop *Property, c Constraint, scalarDef *Default, refDef *RefDefault) PropertyValue {
	ref, obj, hasRef, mixSpan, mixMsg := splitRefTerms(c)
	if mixMsg != "" {
		d := p.errorAt(mixSpan.Start, mixSpan.End, "%s", mixMsg)
		d.Hint = "a reference property takes a reference and/or an object constraint, not scalar comparisons"
	}
	if hasRef || refDef != nil {
		if scalarDef != nil {
			p.errorAt(scalarDef.ValuePos, scalarDef.ValueEnd,
				"property '%s' is a reference; its default must be a reference too", prop.Path)
		}
		return &RefValue{Ref: ref, Object: obj, Default: refDef}
	}
	return &ScalarValue{Constraint: c, Default: scalarDef}
}

// splitRefTerms inspects a constraint expression for reference and object
// terms. It returns the reference and object (if any), whether the
// expression is reference-valued, and, for invalid mixes, the offending
// span and a message.
func splitRefTerms(c Constraint) (ref *Reference, obj *ObjectConstraint, hasRef bool, mixSpan Span, mixMsg string) {
	switch t := c.(type) {
	case nil:
		return nil, nil, false, Span{}, ""
	case *Reference:
		return t, nil, true, Span{}, ""
	case *ObjectConstraint:
		return nil, t, true, Span{}, ""
	case *AndConstraint:
		hasScalar := false
		for _, term := range t.Terms {
			switch tt := term.(type) {
			case *Reference:
				if ref != nil {
					return ref, obj, true, tt.span(), "a property cannot have more than one reference"
				}
				ref = tt
			case *ObjectConstraint:
				if obj != nil {
					return ref, obj, true, tt.span(), "a property cannot have more than one object constraint"
				}
				obj = tt
			default:
				hasScalar = true
			}
		}
		if (ref != nil || obj != nil) && hasScalar {
			return ref, obj, true, c.span(), "a reference cannot be combined with scalar constraints"
		}
		if ref != nil || obj != nil {
			return ref, obj, true, Span{}, ""
		}
		return nil, nil, false, Span{}, ""
	case *OrConstraint:
		for _, alt := range t.Alts {
			if _, _, has, _, _ := splitRefTerms(alt); has {
				return nil, nil, true, t.span(), "a reference cannot appear in a '|' alternative"
			}
		}
		return nil, nil, false, Span{}, ""
	default:
		return nil, nil, false, Span{}, ""
	}
}

// --- values ---

func (p *parser) parseValue(context string) (Value, Span, bool) {
	t := p.cur()
	switch t.kind {
	case tokMinus:
		p.advance()
		num := p.cur()
		if num.kind != tokNumber {
			p.errorAtToken(num, "expected a number after '-', found %s", num.describe())
			return Value{}, t.span(), false
		}
		v, span, ok := p.parseValue(context)
		if !ok {
			return v, span, false
		}
		v.Num = -v.Num
		return v, Span{Start: t.pos, End: span.End}, true

	case tokNumber:
		p.advance()
		v, ok := p.numberValue(t)
		return v, t.span(), ok

	case tokString:
		p.advance()
		return String(t.str), t.span(), true

	case tokTrue:
		p.advance()
		return Bool(true), t.span(), true

	case tokFalse:
		p.advance()
		return Bool(false), t.span(), true

	case tokIdent:
		p.advance()
		d := p.errorAtToken(t, "string values must be quoted")
		d.Hint = fmt.Sprintf("write %q", t.str)
		return String(t.str), t.span(), true

	default:
		if t.kind.isKeyword() {
			d := p.errorAtToken(t, "'%s' is a reserved keyword and cannot be used as a value", t.text)
			d.Hint = fmt.Sprintf("quote it if you meant the string \"%s\"", t.text)
			return Value{}, t.span(), false
		}
		p.errorAtToken(t, "expected a value %s, found %s", context, t.describe())
		return Value{}, t.span(), false
	}
}

// numberValue converts a number token (with optional unit) into a Value.
func (p *parser) numberValue(t token) (Value, bool) {
	switch {
	case t.unit == "":
		return Number(t.num), true
	case sizeUnits[t.unit] != 0:
		v, _ := Size(t.num, t.unit)
		return v, true
	case durationUnits[t.unit] != 0:
		v, _ := Duration(t.num, t.unit)
		return v, true
	default:
		d := p.errorAtToken(t, "unknown unit '%s' in '%s'", t.unit, t.text)
		var all []string
		for u := range sizeUnits {
			all = append(all, u)
		}
		for u := range durationUnits {
			all = append(all, u)
		}
		if s := suggest(t.unit, all); s != "" {
			d.Hint = fmt.Sprintf("did you mean '%s%s'? valid units are %s (size) and %s (duration)",
				formatFloat(t.num), s, unitList(sizeUnits), unitList(durationUnits))
		} else {
			d.Hint = fmt.Sprintf("valid units are %s (size) and %s (duration)",
				unitList(sizeUnits), unitList(durationUnits))
		}
		// Recover as a plain number so parsing can continue.
		return Number(t.num), true
	}
}
