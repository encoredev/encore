package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// predicate represents a stop condition for retry_until. Exactly one field
// should be populated — order of preference: Jq > Path > Status (per spec).
type predicate struct {
	Status int
	Path   *pathPredicate
	Jq     string
}

type pathPredicate struct {
	Path   string
	Equals any
}

// evaluate reports whether the response satisfies the predicate.
func (p predicate) evaluate(status int, body []byte) (bool, error) {
	if p.Jq != "" {
		return evaluateJq(p.Jq, body)
	}
	if p.Path != nil {
		return evaluatePath(*p.Path, body)
	}
	if p.Status != 0 {
		return status == p.Status, nil
	}
	// No predicate set — never matches (caller should treat as "always retry").
	return false, nil
}

var jqLengthRe = regexp.MustCompile(`^(\.[A-Za-z0-9_.]*)\s*\|\s*length\s*(>=|<=|==|!=|>|<)\s*(-?\d+)$`)

func evaluateJq(expr string, body []byte) (bool, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false, fmt.Errorf("empty jq expression")
	}

	// Length form: <path> | length <op> <int>
	if m := jqLengthRe.FindStringSubmatch(expr); m != nil {
		pathStr, op, nStr := m[1], m[2], m[3]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			return false, fmt.Errorf("invalid integer in jq: %q", nStr)
		}
		val, ok := traverseDocBytes(body, pathStr)
		if !ok {
			return false, nil
		}
		length, ok := lengthOf(val)
		if !ok {
			return false, nil
		}
		return compareInt(length, op, n), nil
	}

	// Truthy form: <path> only
	if strings.HasPrefix(expr, ".") && !strings.ContainsAny(expr, "|()[]{}") && !strings.Contains(expr, "==") && !strings.Contains(expr, "!=") {
		val, ok := traverseDocBytes(body, expr)
		if !ok {
			return false, nil
		}
		return truthy(val), nil
	}

	return false, fmt.Errorf("unsupported jq expression: %q (supported: '<path>', '<path> | length <op> N')", expr)
}

func traverseDocBytes(body []byte, dotPath string) (any, bool) {
	if !strings.HasPrefix(dotPath, ".") {
		return nil, false
	}
	parts := strings.Split(strings.TrimPrefix(dotPath, "."), ".")
	if len(parts) == 1 && parts[0] == "" {
		// Top-level identity ("." alone)
		var doc any
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, false
		}
		return doc, true
	}
	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, false
	}
	return traverseDocPath(doc, parts)
}

func lengthOf(v any) (int, bool) {
	switch x := v.(type) {
	case []any:
		return len(x), true
	case map[string]any:
		return len(x), true
	case string:
		return len(x), true
	default:
		return 0, false
	}
}

func compareInt(a int, op string, b int) bool {
	switch op {
	case ">":
		return a > b
	case ">=":
		return a >= b
	case "<":
		return a < b
	case "<=":
		return a <= b
	case "==":
		return a == b
	case "!=":
		return a != b
	}
	return false
}

func truthy(v any) bool {
	if v == nil {
		return false
	}
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}

func evaluatePath(p pathPredicate, body []byte) (bool, error) {
	if !strings.HasPrefix(p.Path, ".") {
		return false, fmt.Errorf("path must start with '.': %q", p.Path)
	}
	parts := strings.Split(strings.TrimPrefix(p.Path, "."), ".")

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		return false, nil
	}
	val, ok := traverseDocPath(doc, parts)
	if !ok {
		return false, nil
	}
	return reflect.DeepEqual(val, p.Equals), nil
}

func traverseDocPath(doc any, parts []string) (any, bool) {
	cur := doc
	for _, part := range parts {
		if part == "" {
			return nil, false
		}
		switch v := cur.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}
