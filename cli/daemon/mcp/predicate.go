package mcp

import (
	"encoding/json"
	"fmt"
	"reflect"
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

// evaluateJq is implemented in a subsequent task.
func evaluateJq(expr string, body []byte) (bool, error) { return false, nil }

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
