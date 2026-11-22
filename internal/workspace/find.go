package workspace

import "strings"

// findLimit caps how many matches FindIn returns: the find panel builds a row
// per match every time they change, and a phrase in a huge document must not
// stall the window.
const findLimit = 500

// Match is one occurrence of a phrase in a document's source: the line it sits
// on, from 1, its column within the line, from 1 in runes, the offset of its
// first byte in the source, and the line's text as a find panel lists it.
type Match struct {
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
	Text   string `json:"text"`
}

// FindIn returns where query appears in src, ignoring case, at most findLimit
// places, and whether it left any out: literal text, not a pattern, as an
// editor's find bar takes it.
func FindIn(src, query string) (matches []Match, truncated bool) {
	needle := []rune(query)
	if len(needle) == 0 || strings.TrimSpace(query) == "" {
		return nil, false
	}
	lines := strings.SplitAfter(src, "\n")
	offset := 0
	for lineNo, line := range lines {
		text := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		runes := []rune(text)
		for i := 0; i+len(needle) <= len(runes); i++ {
			if !strings.EqualFold(string(runes[i:i+len(needle)]), query) {
				continue
			}
			if len(matches) == findLimit {
				// One more than the cap fits: the rest are left out.
				return matches, true
			}
			matches = append(matches, Match{
				Line:   lineNo + 1,
				Col:    i + 1,
				Offset: offset + len(string(runes[:i])),
				Text:   strings.TrimSpace(text),
			})
		}
		offset += len(line)
	}
	return matches, false
}
