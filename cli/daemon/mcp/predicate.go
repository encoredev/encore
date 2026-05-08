package mcp

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

// evaluatePath / evaluateJq are implemented in subsequent tasks.
func evaluatePath(p pathPredicate, body []byte) (bool, error) { return false, nil }
func evaluateJq(expr string, body []byte) (bool, error)       { return false, nil }
