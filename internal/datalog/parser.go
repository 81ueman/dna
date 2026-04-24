package datalog

import (
	"fmt"
	"net/netip"
	"strings"
	"unicode"
)

func ParseProgram(name string, src []byte) (*Program, error) {
	_ = name

	statements, err := splitStatements(stripComments(string(src)))
	if err != nil {
		return nil, err
	}

	program := &Program{Schema: Schema{Relations: map[string]RelationDecl{}}}
	for _, stmt := range statements {
		switch {
		case strings.HasPrefix(stmt, "relation "):
			decl, err := parseRelationDecl(stmt)
			if err != nil {
				return nil, err
			}
			if _, exists := program.Schema.Relations[decl.Name]; exists {
				return nil, fmt.Errorf("duplicate relation %q", decl.Name)
			}
			program.Schema.Relations[decl.Name] = decl
		case strings.Contains(stmt, ":-"):
			rule, err := parseRule(stmt)
			if err != nil {
				return nil, err
			}
			program.Rules = append(program.Rules, rule)
		default:
			return nil, fmt.Errorf("unsupported statement %q", stmt)
		}
	}
	if err := validateSchema(program.Schema); err != nil {
		return nil, err
	}
	if err := validateProgram(program); err != nil {
		return nil, err
	}
	return program, nil
}

func stripComments(src string) string {
	var out strings.Builder
	inString := false
	for _, line := range strings.Split(src, "\n") {
		inString = false
		for i, r := range line {
			if r == '"' && (i == 0 || line[i-1] != '\\') {
				inString = !inString
			}
			if r == '#' && !inString {
				break
			}
			out.WriteRune(r)
		}
		out.WriteByte('\n')
	}
	return out.String()
}

func splitStatements(src string) ([]string, error) {
	var statements []string
	var current strings.Builder
	inString := false
	escaped := false
	for _, r := range src {
		if inString {
			current.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
			current.WriteRune(r)
		case '.':
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string literal")
	}
	if tail := strings.TrimSpace(current.String()); tail != "" {
		return nil, fmt.Errorf("statement missing trailing period: %q", tail)
	}
	return statements, nil
}

func parseRelationDecl(stmt string) (RelationDecl, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(stmt, "relation "))
	name, args, err := parseCall(rest)
	if err != nil {
		return RelationDecl{}, fmt.Errorf("parse relation declaration: %w", err)
	}
	if !isIdent(name) {
		return RelationDecl{}, fmt.Errorf("invalid relation name %q", name)
	}

	decl := RelationDecl{Name: name}
	for _, arg := range args {
		fieldName, rawType, ok := strings.Cut(arg, ":")
		if !ok {
			return RelationDecl{}, fmt.Errorf("relation %q field %q must use name: type", name, arg)
		}
		fieldName = strings.TrimSpace(fieldName)
		rawType = strings.TrimSpace(rawType)
		if !isIdent(fieldName) {
			return RelationDecl{}, fmt.Errorf("relation %q has invalid field name %q", name, fieldName)
		}
		kind := ValueKind(rawType)
		if !validKind(kind) {
			return RelationDecl{}, fmt.Errorf("relation %q field %q has unsupported type %q", name, fieldName, rawType)
		}
		decl.Fields = append(decl.Fields, Field{Name: fieldName, Kind: kind})
	}
	return decl, nil
}

func parseRule(stmt string) (Rule, error) {
	outsideStrings, err := outsideStringText(stmt)
	if err != nil {
		return Rule{}, err
	}
	if strings.Contains(outsideStrings, "!") || containsWord(outsideStrings, "not") {
		return Rule{}, fmt.Errorf("unsupported negation in rule %q", stmt)
	}
	lower := strings.ToLower(outsideStrings)
	if strings.Contains(lower, "aggregate") || strings.Contains(lower, "group<") {
		return Rule{}, fmt.Errorf("unsupported aggregation in rule %q", stmt)
	}
	headRaw, bodyRaw, ok := cutRuleSeparator(stmt)
	if !ok {
		return Rule{}, fmt.Errorf("rule missing :-")
	}
	head, err := parseAtom(strings.TrimSpace(headRaw))
	if err != nil {
		return Rule{}, fmt.Errorf("parse rule head: %w", err)
	}
	bodyParts, err := splitTopLevel(bodyRaw, ',')
	if err != nil {
		return Rule{}, err
	}
	if len(bodyParts) == 0 {
		return Rule{}, fmt.Errorf("rule body must contain at least one atom")
	}
	rule := Rule{Head: head}
	for _, part := range bodyParts {
		atom, err := parseAtom(strings.TrimSpace(part))
		if err != nil {
			return Rule{}, fmt.Errorf("parse rule body: %w", err)
		}
		rule.Body = append(rule.Body, atom)
	}
	return rule, nil
}

func parseAtom(raw string) (Atom, error) {
	name, args, err := parseCall(raw)
	if err != nil {
		return Atom{}, err
	}
	if !isIdent(name) {
		return Atom{}, fmt.Errorf("invalid relation name %q", name)
	}
	atom := Atom{Relation: name}
	for _, arg := range args {
		term, err := parseTerm(arg)
		if err != nil {
			return Atom{}, err
		}
		atom.Terms = append(atom.Terms, term)
	}
	return atom, nil
}

func parseCall(raw string) (string, []string, error) {
	open := strings.IndexRune(raw, '(')
	close := strings.LastIndex(raw, ")")
	if open < 0 || close != len(raw)-1 || close < open {
		return "", nil, fmt.Errorf("%q must be in Name(...) form", raw)
	}
	name := strings.TrimSpace(raw[:open])
	body := strings.TrimSpace(raw[open+1 : close])
	if body == "" {
		return name, nil, nil
	}
	args, err := splitTopLevel(body, ',')
	if err != nil {
		return "", nil, err
	}
	return name, args, nil
}

func parseTerm(raw string) (Term, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Term{}, fmt.Errorf("empty term")
	}
	if raw == "_" {
		return Term{}, fmt.Errorf("unsupported anonymous variable")
	}
	if raw == "true" {
		return BoolLit(true), nil
	}
	if raw == "false" {
		return BoolLit(false), nil
	}
	if strings.HasPrefix(raw, "\"") {
		if !strings.HasSuffix(raw, "\"") || len(raw) < 2 {
			return Term{}, fmt.Errorf("unterminated string literal %q", raw)
		}
		return StringLit(strings.ReplaceAll(raw[1:len(raw)-1], `\"`, `"`)), nil
	}
	if !isIdent(raw) {
		return Term{}, fmt.Errorf("unsupported term %q", raw)
	}
	return Var(raw), nil
}

func outsideStringText(raw string) (string, error) {
	var out strings.Builder
	inString := false
	escaped := false
	for _, r := range raw {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		out.WriteRune(r)
	}
	if inString {
		return "", fmt.Errorf("unterminated string literal")
	}
	return out.String(), nil
}

func cutRuleSeparator(raw string) (string, string, bool) {
	inString := false
	escaped := false
	for i, r := range raw {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		if r == ':' && strings.HasPrefix(raw[i:], ":-") {
			return raw[:i], raw[i+2:], true
		}
	}
	return "", "", false
}

func containsWord(raw, word string) bool {
	for start := 0; start <= len(raw)-len(word); start++ {
		if raw[start:start+len(word)] != word {
			continue
		}
		beforeOK := start == 0 || !isIdentRune(rune(raw[start-1]))
		after := start + len(word)
		afterOK := after == len(raw) || !isIdentRune(rune(raw[after]))
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func splitTopLevel(raw string, sep rune) ([]string, error) {
	var parts []string
	var current strings.Builder
	depth := 0
	inString := false
	escaped := false
	for _, r := range raw {
		if inString {
			current.WriteRune(r)
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		switch r {
		case '"':
			inString = true
			current.WriteRune(r)
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parentheses")
			}
			current.WriteRune(r)
		default:
			if r == sep && depth == 0 {
				part := strings.TrimSpace(current.String())
				if part == "" {
					return nil, fmt.Errorf("empty list element")
				}
				parts = append(parts, part)
				current.Reset()
				continue
			}
			current.WriteRune(r)
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string literal")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parentheses")
	}
	part := strings.TrimSpace(current.String())
	if part == "" {
		return nil, fmt.Errorf("empty list element")
	}
	parts = append(parts, part)
	return parts, nil
}

func validateProgram(program *Program) error {
	for _, rule := range program.Rules {
		if err := validateAtom(program.Schema, rule.Head); err != nil {
			return fmt.Errorf("head %s: %w", rule.Head.Relation, err)
		}
		bodyVars := map[string]ValueKind{}
		for _, atom := range rule.Body {
			if err := validateAtom(program.Schema, atom); err != nil {
				return fmt.Errorf("body %s: %w", atom.Relation, err)
			}
			decl := program.Schema.Relations[atom.Relation]
			for i, term := range atom.Terms {
				kind := decl.Fields[i].Kind
				if term.IsVariable() {
					if previous, ok := bodyVars[term.Variable]; ok && previous != kind {
						return fmt.Errorf("variable %q used with both %s and %s", term.Variable, previous, kind)
					}
					bodyVars[term.Variable] = kind
					continue
				}
				if err := literalMatches(term.Literal, kind); err != nil {
					return fmt.Errorf("body %s term %d: %w", atom.Relation, i, err)
				}
			}
		}

		headDecl := program.Schema.Relations[rule.Head.Relation]
		for i, term := range rule.Head.Terms {
			kind := headDecl.Fields[i].Kind
			if term.IsVariable() {
				bodyKind, ok := bodyVars[term.Variable]
				if !ok {
					return fmt.Errorf("head variable %q is not bound by rule body", term.Variable)
				}
				if bodyKind != kind {
					return fmt.Errorf("head variable %q has type %s, target field requires %s", term.Variable, bodyKind, kind)
				}
				continue
			}
			if err := literalMatches(term.Literal, kind); err != nil {
				return fmt.Errorf("head %s term %d: %w", rule.Head.Relation, i, err)
			}
		}
	}
	return validateAcyclicRules(program)
}

func validateAtom(schema Schema, atom Atom) error {
	decl, ok := schema.Relations[atom.Relation]
	if !ok {
		return fmt.Errorf("unknown relation %q", atom.Relation)
	}
	if len(atom.Terms) != len(decl.Fields) {
		return fmt.Errorf("arity mismatch: got %d terms, want %d", len(atom.Terms), len(decl.Fields))
	}
	return nil
}

func literalMatches(lit Literal, kind ValueKind) error {
	if !lit.Set {
		return fmt.Errorf("missing literal")
	}
	switch kind {
	case KindString:
		if lit.Kind != KindString {
			return fmt.Errorf("literal has type %s, want %s", lit.Kind, kind)
		}
	case KindBool:
		if lit.Kind != KindBool {
			return fmt.Errorf("literal has type %s, want %s", lit.Kind, kind)
		}
	case KindAddr:
		if lit.Kind != KindString {
			return fmt.Errorf("addr literal must be quoted")
		}
		if _, err := netip.ParseAddr(lit.String); err != nil {
			return fmt.Errorf("invalid addr literal %q", lit.String)
		}
	case KindPrefix:
		if lit.Kind != KindString {
			return fmt.Errorf("prefix literal must be quoted")
		}
		if _, err := netip.ParsePrefix(lit.String); err != nil {
			return fmt.Errorf("invalid prefix literal %q", lit.String)
		}
	default:
		return fmt.Errorf("unsupported type %s", kind)
	}
	return nil
}

func validateAcyclicRules(program *Program) error {
	graph := map[string][]string{}
	for _, rule := range program.Rules {
		for _, atom := range rule.Body {
			graph[rule.Head.Relation] = append(graph[rule.Head.Relation], atom.Relation)
		}
	}

	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(relation string) error {
		if visiting[relation] {
			return fmt.Errorf("unsupported recursive rule involving %q", relation)
		}
		if visited[relation] {
			return nil
		}
		visiting[relation] = true
		for _, dep := range graph[relation] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[relation] = false
		visited[relation] = true
		return nil
	}
	for relation := range graph {
		if err := visit(relation); err != nil {
			return err
		}
	}
	return nil
}

func isIdent(raw string) bool {
	if raw == "" {
		return false
	}
	for i, r := range raw {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
