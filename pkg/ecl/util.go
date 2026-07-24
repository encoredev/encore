package ecl

import "strings"

// suggest returns the candidate closest to s, or "" if nothing is close
// enough to be a plausible typo. Among equally close candidates, those
// sharing a prefix with s are preferred (so "G" suggests "GB", not "B").
func suggest(s string, candidates []string) string {
	type score struct {
		dist     int
		noPrefix bool
	}
	best, bestScore := "", score{dist: 3} // only suggest within edit distance 2
	better := func(a, b score) bool {
		if a.dist != b.dist {
			return a.dist < b.dist
		}
		return !a.noPrefix && b.noPrefix
	}
	lower := strings.ToLower(s)
	for _, c := range candidates {
		if c == s {
			continue
		}
		cl := strings.ToLower(c)
		sc := score{
			dist:     editDistance(lower, cl),
			noPrefix: !strings.HasPrefix(cl, lower) && !strings.HasPrefix(lower, cl),
		}
		if better(sc, bestScore) || (sc == bestScore && best != "" && c < best) {
			best, bestScore = c, sc
		}
	}
	if best != "" && bestScore.dist > len(s) {
		return "" // suggestion would replace the whole input
	}
	return best
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, min(cur[j-1]+1, prev[j-1]+cost))
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
