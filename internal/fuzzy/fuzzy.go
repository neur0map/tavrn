package fuzzy

import "strings"

// Score returns a match quality score. Negative means no match.
// Higher is better: exact > prefix > contiguous substring > fuzzy.
func Score(query, target string) int {
	if query == "" {
		return 0
	}

	q := strings.ToLower(query)
	t := strings.ToLower(target)

	// Exact match
	if q == t {
		return 300
	}

	// Prefix match
	if strings.HasPrefix(t, q) {
		return 200 + len(q)
	}

	// Contiguous substring
	if strings.Contains(t, q) {
		return 100 + len(q)
	}

	// Fuzzy: characters appear in order
	qi := 0
	for ti := 0; ti < len(t) && qi < len(q); ti++ {
		if t[ti] == q[qi] {
			qi++
		}
	}
	if qi == len(q) {
		return qi
	}

	return -1
}
