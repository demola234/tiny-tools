package suggest

import "unicode/utf8"

const maxEdits = 2

func Closest(got string, candidates []string) (string, bool) {
	best, bestDist := "", maxEdits+1
	g := []rune(got)
	for _, c := range candidates {
		if abs(utf8.RuneCountInString(c)-len(g)) > maxEdits {
			continue
		}
		if d := within(g, []rune(c), maxEdits); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best, bestDist <= maxEdits
}

func Distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	return within(ra, rb, len(ra)+len(rb))
}

func within(a, b []rune, limit int) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		rowMin := curr[0]
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
			rowMin = min(rowMin, curr[j])
		}
		if rowMin > limit {
			return limit + 1
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
