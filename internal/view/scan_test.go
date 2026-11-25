package view

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/watch"
	"mdpreview/internal/workspace"
)

// waitFor drains what the window's goroutines posted until cond holds, so that
// a test of the scan that runs on one waits for it rather than sleeping.
func waitFor(t *testing.T, s *State, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.drain()
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("the window never got there")
}

// heldScan replaces the scan with one the test lets go when it chooses, and
// returns the release, the channel a scan announces itself on, and how many
// scans started.
func heldScan(s *State) (release chan struct{}, started chan struct{}, scans *atomic.Int64) {
	release, started, scans = make(chan struct{}), make(chan struct{}, 8), &atomic.Int64{}
	s.syncScan = false
	s.scan = func(p workspace.Project) (*workspace.Node, error) {
		scans.Add(1)
		started <- struct{}{}
		<-release
		return workspace.BuildTree(p.Root)
	}
	return release, started, scans
}

func startScanNow(t *testing.T, started chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the scan never started")
	}
}

// hasChild reports whether the tree has an entry of that name.
func hasChild(n *workspace.Node, name string) bool {
	if n == nil {
		return false
	}
	for _, c := range n.Children {
		if c.Name == name {
			return true
		}
	}
	return false
}

func TestTheTreeLoadsOnAGoroutine(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "later/a.md", "# A\n")
	p := s.Projects()[0]
	release, started, _ := heldScan(s)
	s.Refresh()
	startScanNow(t, started)
	// While the scan runs the frame still has the tree the window had.
	if !s.scanning[p.ID] {
		t.Error("scanning = false, want the scan in flight")
	}
	if trees := s.Trees(); len(trees) != 1 || hasChild(trees[0].root, "later") {
		t.Errorf("Trees = %v, want the tree the window had before the scan", trees)
	}
	close(release)
	waitFor(t, s, func() bool { return !s.scanning[p.ID] })
	if trees := s.Trees(); len(trees) != 1 || !hasChild(trees[0].root, "later") {
		t.Errorf("Trees = %v, want the scanned tree", trees)
	}
}

func TestTheProjectShowsASpinnerWhileScanning(t *testing.T) {
	s, _ := newState(t)
	p := s.Projects()[0]
	release, started, _ := heldScan(s)
	s.Refresh()
	startScanNow(t, started)
	tt := newTester(s)
	// The spinner sits inside the project's button, whose own name the button's
	// label owns: what a screen reader reads of it is that the button is busy.
	if a, ok := tt.Access("切换项目"); !ok || !a.Busy {
		t.Errorf("project button = %+v, %v, want it busy while the tree is read", a, ok)
	}
	close(release)
	waitFor(t, s, func() bool { return !s.scanning[p.ID] })
	tt = newTester(s)
	if a, ok := tt.Access("切换项目"); ok && a.Busy {
		t.Error("the button is still busy after the scan")
	}
}

func TestTheTreeSaysWhatItIsScanning(t *testing.T) {
	s, _ := newState(t)
	p := s.Projects()[0]
	s.trees = nil
	s.scanning = map[string]bool{p.ID: true}
	tt := newTester(s)
	if !tt.HasText("正在扫描 alpha…") {
		t.Errorf("texts = %q, want the scan named", tt.Texts())
	}
	s.scanning = map[string]bool{}
	tt = newTester(s)
	if !tt.HasText("打开一个目录或 Markdown 文件") {
		t.Errorf("texts = %q, want the way to open a project", tt.Texts())
	}
}

func TestALateScanDoesNotResurrectAClosedProject(t *testing.T) {
	s, _ := newState(t)
	p := s.Projects()[0]
	release, started, _ := heldScan(s)
	s.Refresh()
	startScanNow(t, started)
	s.ToggleProject(p.ID)
	if len(s.Trees()) != 0 {
		t.Fatalf("Trees = %v, want none: the project is closed", s.Trees())
	}
	close(release)
	waitFor(t, s, func() bool { return !s.scanning[p.ID] })
	if len(s.Trees()) != 0 {
		t.Errorf("Trees = %v, want none: a scan of a closed project changes nothing", s.Trees())
	}
}

func TestScanningAgainKeepsOneResult(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	p := s.Projects()[0]
	release, started, scans := heldScan(s)
	s.Refresh()
	startScanNow(t, started)
	// The tree changed twice while the first scan ran: one more scan covers
	// both, and the window keeps one tree.
	s.Refresh()
	s.Refresh()
	close(release)
	waitFor(t, s, func() bool { return scans.Load() >= 2 && !s.scanning[p.ID] })
	waitFor(t, s, func() bool { return countsOneTree(s) })
	if got := scans.Load(); got != 2 {
		t.Errorf("scans = %d, want 2: the asks of one scan in flight are merged", got)
	}
	if trees := s.Trees(); len(trees) != 1 || !hasChild(trees[0].root, "a.md") {
		t.Errorf("Trees = %v, want the one tree", trees)
	}
}

// countsOneTree reports whether the one open project shows one document.
func countsOneTree(s *State) bool {
	trees := s.Trees()
	return len(trees) == 1 && hasChild(trees[0].root, "a.md") && !s.scanning[trees[0].project.ID]
}

func TestATooLargeDirectoryIsRefusedWithANotice(t *testing.T) {
	s, _ := newState(t)
	p := s.Projects()[0]
	var scans atomic.Int64
	s.scan = func(workspace.Project) (*workspace.Node, error) {
		scans.Add(1)
		return nil, workspace.ErrTreeTooLarge
	}
	s.Refresh()
	if !s.tooLarge[p.ID] {
		t.Fatal("tooLarge = false, want the project refused")
	}
	if trees := s.Trees(); len(trees) != 0 {
		t.Errorf("Trees = %v, want the refused project out of the tree", trees)
	}
	if got := s.Status(); !strings.Contains(got, "目录过大无法打开，请缩小目录范围") {
		t.Errorf("Status = %q, want the refusal", got)
	}
	// A refused project is not read again on every frame: only the user's
	// reload asks again.
	s.reloadTree()
	if got := scans.Load(); got != 1 {
		t.Errorf("scans = %d, want 1: a refused project is not read again", got)
	}
}

func TestRefreshingATooLargeDirectoryTriesAgain(t *testing.T) {
	s, root := newState(t)
	writeDoc(t, root, "a.md", "# A\n")
	p := s.Projects()[0]
	s.scan = func(workspace.Project) (*workspace.Node, error) { return nil, workspace.ErrTreeTooLarge }
	s.Refresh()
	if !s.tooLarge[p.ID] {
		t.Fatal("tooLarge = false, want the project refused")
	}
	s.scan = func(p workspace.Project) (*workspace.Node, error) { return workspace.BuildTree(p.Root) }
	s.Refresh()
	if s.tooLarge[p.ID] {
		t.Error("tooLarge = true, want the reload to try again")
	}
	if trees := s.Trees(); len(trees) != 1 || !hasChild(trees[0].root, "a.md") {
		t.Errorf("Trees = %v, want the project back", trees)
	}
}

func TestAWatchLimitStopsTheWatchAndSaysSo(t *testing.T) {
	s, _ := newState(t)
	p := s.Projects()[0]
	if got := watchFailure(p, mygo.ErrWatchLimit); got != "目录过大，已停止监听：alpha（刷新仍可用）" {
		t.Errorf("watchFailure(limit) = %q, want the ceiling named", got)
	}
	if got := watchFailure(p, errors.New("boom")); got != "无法监听这个项目：boom" {
		t.Errorf("watchFailure(other) = %q, want the error named", got)
	}
}

// accessSays reports whether anything the window tells a screen reader holds
// that phrase: a tree item's name is what its row holds, all of it.
func accessSays(tt *ui.Tester, phrase string) bool {
	for _, a := range tt.AccessAll() {
		if strings.Contains(a.Name, phrase) {
			return true
		}
	}
	return false
}

// accessLabels is what a screen reader would read of the window, for a test
// that expected one of them.
func accessLabels(tt *ui.Tester) []string {
	all := tt.AccessAll()
	labels := make([]string, 0, len(all))
	for _, a := range all {
		labels = append(labels, a.Name)
	}
	return labels
}

// TestOpeningAProjectShowsItBeforeTheScanEnds pins the row the window shows at
// once: the project appears in the tree, marked as being read, rather than
// waiting for the scan of a directory of thousands of entries.
func TestOpeningAProjectShowsItBeforeTheScanEnds(t *testing.T) {
	s, alphaRoot, _ := twoProjects(t)
	writeDoc(t, alphaRoot, "a.md", "# A\n")
	s.Refresh()
	release, started, _ := heldScan(s)
	dir := t.TempDir()
	s.AddProject(dir)
	startScanNow(t, started)
	name := filepath.Base(dir)
	tt := newTester(s)
	if !tt.HasText(name) {
		t.Errorf("texts = %q, want the project's node at once", tt.Texts())
	}
	// A tree item's accessible name is what its row holds, the spinner's own
	// label among it.
	if !accessSays(tt, "正在扫描 "+name) {
		t.Errorf("access = %v, want the project's row to say it is being read", accessLabels(tt))
	}
	id := s.Projects()[len(s.Projects())-1].ID
	close(release)
	waitFor(t, s, func() bool { return !s.scanning[id] })
	tt = newTester(s)
	if accessSays(tt, "正在扫描 "+name) {
		t.Error("the spinner outlived the scan")
	}
}

// TestOpeningAProjectDoesNotWaitForItsWatch pins the frame: a watch reads the
// project it covers before it reports anything, and a directory of thousands of
// entries takes long enough that doing it in the frame would freeze the window.
func TestOpeningAProjectDoesNotWaitForItsWatch(t *testing.T) {
	s, _ := newState(t)
	s.watchReady = true
	release, started := make(chan struct{}), make(chan struct{}, 1)
	s.watchProject = func(context.Context, workspace.Project) (watch.Source, error) {
		started <- struct{}{}
		<-release
		return nil, errors.New("held")
	}
	dir := t.TempDir()
	writeDoc(t, dir, "a.md", "# A\n")
	done := make(chan struct{})
	go func() {
		s.AddProject(dir)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("AddProject waited for the watch")
	}
	startScanNow(t, started)
	if tt := newTester(s); !tt.HasText(filepath.Base(dir)) {
		t.Errorf("texts = %q, want the project in the tree", tt.Texts())
	}
	close(release)
}
