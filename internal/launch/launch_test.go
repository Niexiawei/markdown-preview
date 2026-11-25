package launch

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// doc writes a markdown file into dir and returns its path.
func doc(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("# "+name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDocumentsPicksMarkdownArguments(t *testing.T) {
	dir := t.TempDir()
	a := doc(t, dir, "a.md")
	b := doc(t, dir, "B.MARKDOWN")
	doc(t, dir, "c.txt")
	got := Documents([]string{a, b, filepath.Join(dir, "c.txt")}, dir)
	if len(got) != 2 || got[0] != a || got[1] != b {
		t.Errorf("Documents = %v, want [%s %s]", got, a, b)
	}
}

func TestDocumentsJoinsRelativeArgumentsToTheWorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	a := doc(t, dir, "notes/a.md")
	got := Documents([]string{filepath.Join("notes", "a.md")}, dir)
	if len(got) != 1 || got[0] != a {
		t.Errorf("Documents = %v, want [%s]", got, a)
	}
}

func TestDocumentsTakesFileURLs(t *testing.T) {
	dir := t.TempDir()
	a := doc(t, dir, "a.md")
	link := (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(a)}).String()
	got := Documents([]string{link}, t.TempDir())
	if len(got) != 1 || got[0] != a {
		t.Errorf("Documents(%q) = %v, want [%s]", link, got, a)
	}
	// Windows writes a file URL with the drive as its host: file://C:/notes.
	if filepath.VolumeName(a) != "" {
		drive := "file://" + filepath.ToSlash(a)
		if got := Documents([]string{drive}, t.TempDir()); len(got) != 1 || got[0] != a {
			t.Errorf("Documents(%q) = %v, want [%s]", drive, got, a)
		}
	}
}

func TestDocumentsIgnoresWhatIsNotAFile(t *testing.T) {
	dir := t.TempDir()
	doc(t, dir, "a.md")
	cases := [][]string{
		{filepath.Join(dir, "gone.md")},
		{dir},
		{filepath.Join(dir, "noext")},
		{"-v"},
		{""},
	}
	for _, args := range cases {
		if got := Documents(args, dir); len(got) != 0 {
			t.Errorf("Documents(%q) = %v, want nothing", args, got)
		}
	}
}
