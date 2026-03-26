package ui

import (
	"strings"
	"unicode"
)

type dirEntry struct {
	name string
}

type scoredEntry struct {
	name        string
	score       int
	matchRanges [][2]int
}

// fuzzyMatch does a subsequence match of pattern against name.
// Returns score > 0 if matched, and the character ranges that matched.
func fuzzyMatch(name, pattern string) (int, [][2]int) {
	nameLower := strings.ToLower(name)
	patLower := strings.ToLower(pattern)

	// Find matched positions
	var positions []int
	pi := 0
	for ni := 0; ni < len(nameLower) && pi < len(patLower); ni++ {
		if nameLower[ni] == patLower[pi] {
			positions = append(positions, ni)
			pi++
		}
	}
	if pi < len(patLower) {
		return 0, nil // not a subsequence
	}

	// Score
	score := len(positions) // base: 1 per matched char

	for i, pos := range positions {
		// Bonus: match at start
		if pos == 0 {
			score += 10
		}
		// Bonus: word boundary
		if pos > 0 && isWordBoundary(rune(name[pos-1]), rune(name[pos])) {
			score += 5
		}
		// Bonus: consecutive match
		if i > 0 && positions[i-1] == pos-1 {
			score += 3
		}
	}

	// Bonus: exact prefix
	if strings.HasPrefix(nameLower, patLower) {
		score += 20
	}

	// Build ranges from positions (merge consecutive)
	var ranges [][2]int
	start := positions[0]
	end := positions[0] + 1
	for i := 1; i < len(positions); i++ {
		if positions[i] == end {
			end++
		} else {
			ranges = append(ranges, [2]int{start, end})
			start = positions[i]
			end = positions[i] + 1
		}
	}
	ranges = append(ranges, [2]int{start, end})

	return score, ranges
}

func isWordBoundary(prev, curr rune) bool {
	return prev == '_' || prev == '-' || prev == '.' ||
		(unicode.IsLower(prev) && unicode.IsUpper(curr))
}
