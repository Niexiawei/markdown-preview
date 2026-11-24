package workspace

import "strings"

// FuzzyMatch reports whether query is a subsequence of name: its letters, in
// order, with anything between them and in any case. It is what the file tree's
// search matches names with, so a regexp's punctuation is a letter to find
// rather than a pattern to compile, and a blank query matches nothing — the tree
// shows every file when nothing was typed.
func FuzzyMatch(name, query string) bool {
	query = strings.TrimSpace(query)
	if query == "" {
		return false
	}
	want := []rune(strings.ToLower(query))
	at := 0
	for _, r := range strings.ToLower(name) {
		if r != want[at] {
			continue
		}
		at++
		if at == len(want) {
			return true
		}
	}
	return false
}
