package view

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
)

// Rendering the window to PNGs.
//
// This is not an assertion harness: nothing here fails because a pixel moved,
// and it writes nothing unless MDPREVIEW_SHOT_DIR is set. It exists because
// some questions can only be answered by looking — is the fill of a row the
// whole row, is the arrow in the same column as a document's icon, is the
// loading mark small enough, does the tree read like the one the framework
// draws. A view test can prove a text is there; only the picture proves it is
// readable.
//
//	MDPREVIEW_SHOT_DIR=/tmp/shots go test ./internal/view -run TestScreenshots -v
//
// Add a case whenever the sidebar, the tree or a panel gains a state worth
// seeing (see AGENTS.md, 「GUI 验证」).
type shotCase struct {
	name string
	// build returns the window state to capture.
	build func(t *testing.T) *State
	// view defaults to the state's View; set it to capture one slot alone.
	view func(*State) func(*ui.Context)
	// width and height default to 1000x760.
	width, height int
	// settle is how many frames to draw before capturing: a moving loading
	// mark and a panel opening animate in.
	settle int
	// act does something to the frame first, as moving the pointer onto a row
	// or opening the project panel.
	act func(t *testing.T, s *State, tt *ui.Tester)
}

func (s shotCase) frames() int {
	if s.settle > 0 {
		return s.settle
	}
	return 3
}

func (s shotCase) size() (int, int) {
	width, height := s.width, s.height
	if width == 0 {
		width = 1000
	}
	if height == 0 {
		height = 760
	}
	return width, height
}

// TestScreenshots renders every case to a PNG under MDPREVIEW_SHOT_DIR.
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("MDPREVIEW_SHOT_DIR")
	if dir == "" {
		t.Skip("set MDPREVIEW_SHOT_DIR to a directory to render the window as PNGs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create %s: %v", dir, err)
	}
	for _, shot := range shotCases() {
		t.Run(shot.name, func(t *testing.T) {
			s := shot.build(t)
			view := s.View
			if shot.view != nil {
				view = shot.view(s)
			}
			width, height := shot.size()
			tt := ui.NewTester(view, width, height)
			for range shot.frames() {
				// The tester's clock is virtual, so a frame does not let a
				// goroutine land; the sleep is what makes a scan arrive.
				tt.Advance(20 * time.Millisecond)
				tt.Frame()
				time.Sleep(2 * time.Millisecond)
			}
			if shot.act != nil {
				shot.act(t, s, tt)
				tt.Advance(20 * time.Millisecond)
				tt.Frame()
			}
			path := filepath.Join(dir, shot.name+".png")
			writeShot(t, path, tt.Image())
			t.Log(path)
		})
	}
}

// writeShot saves one frame.
func writeShot(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

// shotTree is a window with one project, a directory two levels deep, and a
// document shown, which is what most of the sidebar's states are about.
func shotTree(t *testing.T) *State {
	t.Helper()
	s, root := newState(t)
	writeDoc(t, root, "docs/design/plan.md", "# Plan\n\n## 一\n\n正文\n")
	writeDoc(t, root, "docs/notes.md", "# Notes\n")
	writeDoc(t, root, "CLAUDE.md", "# Claude\n")
	writeDoc(t, root, "README.md", "# Readme\n")
	s.Refresh()
	s.selectDoc("docs/design/plan.md")
	return s
}

// shotCases is every state worth looking at, in reading order.
func shotCases() []shotCase {
	return []shotCase{
		{
			name:  "01-empty",
			build: func(t *testing.T) *State { return newStateWith(config.Config{Version: config.Version}) },
		},
		{
			name:  "02-tree",
			build: shotTree,
		},
		{
			name:  "03-hover",
			build: shotTree,
			act: func(t *testing.T, s *State, tt *ui.Tester) {
				r, ok := tt.Find("README.md")
				if !ok {
					t.Fatalf("no README.md row; texts = %v", tt.Texts())
				}
				tt.Move(r.X+r.W/2, r.Y+r.H/2)
			},
		},
		{
			name: "04-loose-file",
			build: func(t *testing.T) *State {
				s := shotTree(t)
				s.OpenDocument(looseDoc(t, "# Loose\n"))
				return s
			},
		},
		{
			name: "05-search",
			build: func(t *testing.T) *State {
				s := shotTree(t)
				openTreeSearchOn(s, "n")
				s.moveCandidate(0)
				return s
			},
		},
		{
			name: "06-folded",
			build: func(t *testing.T) *State {
				s, _, _ := twoProjects(t)
				writeDoc(t, s.Trees()[0].project.Root, "docs/a.md", "# A\n")
				writeDoc(t, s.Trees()[1].project.Root, "only/b.md", "# B\n")
				s.Refresh()
				s.setTreeOpen("", false)
				return s
			},
		},
		{
			name: "07-scanning",
			build: func(t *testing.T) *State {
				s, _, _ := twoProjects(t)
				writeDoc(t, s.Trees()[0].project.Root, "docs/a.md", "# A\n")
				s.Refresh()
				release, _, _ := heldScan(s)
				t.Cleanup(func() {
					select {
					case <-release:
					default:
						close(release)
					}
				})
				s.AddProject(t.TempDir())
				return s
			},
			settle: 5,
		},
		{
			name: "08-project-panel",
			build: func(t *testing.T) *State {
				s := shotTree(t)
				s.projectsOpen = true
				return s
			},
		},
		{
			name: "09-multi-project",
			build: func(t *testing.T) *State {
				s, alphaRoot, betaRoot := twoProjects(t)
				writeDoc(t, alphaRoot, "docs/a.md", "# A\n")
				writeDoc(t, betaRoot, "notes/b.md", "# B\n")
				s.Refresh()
				s.selectDocIn(s.Trees()[1].project.ID, "notes/b.md")
				return s
			},
		},
		{
			name: "10-sunk-panel",
			build: func(t *testing.T) *State {
				s := shotTree(t)
				s.panelDown = true
				return s
			},
		},
	}
}
