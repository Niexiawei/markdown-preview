package view

import (
	"strconv"
	"testing"
)

// BenchmarkCandidateLookup measures the per-row work of the filtered tree: every
// file of a project asks whether it is the candidate the keyboard is on.
func BenchmarkCandidateLookup(b *testing.B) {
	s := &State{}
	const files, candidates = 3000, 3000
	for i := range candidates {
		s.treeSearch.candidates = append(s.treeSearch.candidates,
			docSel{project: "p", path: "plan_" + strconv.Itoa(i) + ".md"})
	}
	s.treeSearch.open, s.treeSearch.query = true, "plan"
	s.treeSearch.current = candidates - 1
	b.ResetTimer()
	for range b.N {
		for i := range files {
			_ = s.isCurrentCandidate("p", "plan_"+strconv.Itoa(i)+".md")
		}
	}
}
