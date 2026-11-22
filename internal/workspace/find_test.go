package workspace

import (
	"strings"
	"testing"
)

func TestFindInReportsLineAndColumn(t *testing.T) {
	src := "第一行\n第二行 abc\n第三行\nabc 又一次 abc\n"
	got, truncated := FindIn(src, "abc")
	if truncated {
		t.Error("truncated = true, want false for three matches")
	}
	if len(got) != 3 {
		t.Fatalf("FindIn = %v, want three", got)
	}
	want := []struct{ line, col int }{{2, 5}, {4, 1}, {4, 9}}
	for i, w := range want {
		if got[i].Line != w.line || got[i].Col != w.col {
			t.Errorf("match %d = line %d col %d, want line %d col %d", i, got[i].Line, got[i].Col, w.line, w.col)
		}
	}
	if got[0].Text != "第二行 abc" {
		t.Errorf("match 0 text = %q, want the whole line", got[0].Text)
	}
	if got[0].Offset != strings.Index(src, "abc") {
		t.Errorf("match 0 offset = %d, want %d", got[0].Offset, strings.Index(src, "abc"))
	}
}

func TestFindInIgnoresCaseAndEmpties(t *testing.T) {
	src := "IMPORT 表格\nimport 又见\n"
	if got, _ := FindIn(src, "import"); len(got) != 2 {
		t.Errorf("FindIn = %v, want two matches ignoring case", got)
	}
	if got, _ := FindIn(src, "  "); len(got) != 0 {
		t.Errorf("FindIn(blank) = %v, want none", got)
	}
	if got, _ := FindIn(src, ""); len(got) != 0 {
		t.Errorf("FindIn(empty) = %v, want none", got)
	}
}

func TestFindInHandlesCRLF(t *testing.T) {
	got, _ := FindIn("一\r\n二 abc\r\n", "abc")
	if len(got) != 1 || got[0].Line != 2 || got[0].Text != "二 abc" {
		t.Fatalf("FindIn = %+v, want line 2 without the carriage return", got)
	}
}

func TestFindInKeepsByteOffsets(t *testing.T) {
	src := "写字\nabc 在这里\n"
	got, _ := FindIn(src, "abc")
	if len(got) != 1 {
		t.Fatalf("FindIn = %v, want one", got)
	}
	if src[got[0].Offset:] == "" || !strings.HasPrefix(src[got[0].Offset:], "abc") {
		t.Errorf("offset %d points at %q, want the match itself", got[0].Offset, src[got[0].Offset:])
	}
}

func TestFindInCapsResults(t *testing.T) {
	got, truncated := FindIn(strings.Repeat("x y\n", 600), "x")
	if len(got) != findLimit || !truncated {
		t.Errorf("FindIn = %d matches, truncated %v, want the %d cap and the flag", len(got), truncated, findLimit)
	}
	// Exactly the cap's worth, and no more: nothing was left out.
	exact, cut := FindIn(strings.Repeat("x\n", findLimit), "x")
	if len(exact) != findLimit || cut {
		t.Errorf("FindIn = %d matches, truncated %v, want no truncation at exactly the cap", len(exact), cut)
	}
}
