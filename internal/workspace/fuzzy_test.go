package workspace

import "testing"

// TestFuzzyMatchTakesALetterSubsequence pins what the tree's search calls a
// match: the phrase's letters, in order, with anything between them.
func TestFuzzyMatchTakesALetterSubsequence(t *testing.T) {
	for _, name := range []string{"RELEASE_PLAN.md", "p_l_a_n.md", "plan.md"} {
		if !FuzzyMatch(name, "plan") {
			t.Errorf("FuzzyMatch(%q, %q) = false, want true", name, "plan")
		}
	}
}

// TestFuzzyMatchIgnoresCase pins that typing lowercase finds a name spelled
// in capitals, as a search field is expected to.
func TestFuzzyMatchIgnoresCase(t *testing.T) {
	if !FuzzyMatch("release_plan.md", "Plan") {
		t.Errorf("FuzzyMatch(%q, %q) = false, want true", "release_plan.md", "Plan")
	}
	if !FuzzyMatch("PLAN.md", "plan") {
		t.Errorf("FuzzyMatch(%q, %q) = false, want true", "PLAN.md", "plan")
	}
}

// TestFuzzyMatchKeepsTheOrder pins that the letters have to come in the
// phrase's order: a subsequence, not a bag of letters.
func TestFuzzyMatchKeepsTheOrder(t *testing.T) {
	if FuzzyMatch("lm.md", "ml") {
		t.Errorf("FuzzyMatch(%q, %q) = true, want false: the letters came the other way round", "lm.md", "ml")
	}
	if !FuzzyMatch("m_l.md", "ml") {
		t.Errorf("FuzzyMatch(%q, %q) = false, want true", "m_l.md", "ml")
	}
}

// TestFuzzyMatchRejectsAMissingLetter pins that every letter of the phrase has
// to be there: a near miss is no match.
func TestFuzzyMatchRejectsAMissingLetter(t *testing.T) {
	if FuzzyMatch("plan.md", "pnx") {
		t.Errorf("FuzzyMatch(%q, %q) = true, want false", "plan.md", "pnx")
	}
}

// TestFuzzyMatchTreatsPunctuationLiterally pins that the phrase is compared
// letter by letter, not compiled: a regexp's punctuation is just a character
// to find, and a name without it does not match.
func TestFuzzyMatchTreatsPunctuationLiterally(t *testing.T) {
	for _, query := range []string{".", "*", "[", `\`, "(", "a.m"} {
		if FuzzyMatch("plannnmd", query) {
			t.Errorf("FuzzyMatch(%q, %q) = true, want false: %q is not in the name", "plannnmd", query, query)
		}
	}
	if !FuzzyMatch("plan.md", ".") {
		t.Errorf("FuzzyMatch(%q, %q) = false, want true", "plan.md", ".")
	}
}

// TestFuzzyMatchOfAnEmptyQueryIsFalse pins the empty phrase: it is no match for
// anything, since the tree shows every file when nothing was typed.
func TestFuzzyMatchOfAnEmptyQueryIsFalse(t *testing.T) {
	for _, query := range []string{"", " ", "\t"} {
		if FuzzyMatch("plan.md", query) {
			t.Errorf("FuzzyMatch(%q, %q) = true, want false", "plan.md", query)
		}
	}
}
