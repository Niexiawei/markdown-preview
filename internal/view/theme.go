package view

import (
	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/theme"
)

// The colours this app owns on top of Onevra's theme. Everything that needs
// one of them reads it from here, so a palette change is one line.
const (
	accentHex        = "#0052d9"
	accentHoverHex   = "#618dff"
	accentPressedHex = "#003cab"
	// warningHex marks a destructive or cautionary action and a warning badge.
	warningHex = "#e37318"
	// selectionWash is how strongly a picked row's fill washes the accent: the
	// palette's own strength, so that picked rows stay under the document's
	// accent fill rather than competing with it.
	selectionWash float32 = 0.22
)

// AppTheme is Onevra's theme with this app's palette. The shell installs it
// once, so every view and every component reads the same colours.
func AppTheme(c *ui.Context) *ui.Theme {
	t := theme.Onevra(c)
	t.Accent = ui.Hex(accentHex)
	t.AccentHover = ui.Hex(accentHoverHex)
	t.AccentPressed = ui.Hex(accentPressedHex)
	t.Warning = ui.Hex(warningHex)
	// Onevra's palette keeps its own orange for selections; this app's accent
	// is blue, and a picked row has to read as this app's selection, not as
	// another palette's highlight (mygo's own default selection is the accent's
	// too).
	t.Selection = t.Accent.Alpha(selectionWash)
	return t
}

// selectedRowTint is the selected sidebar row's fill: the accent, washed out
// so the text keeps its contrast.
func selectedRowTint(t *ui.Theme) ui.Color {
	return t.Accent.Alpha(0.12)
}

// currentCandidateTint is the fill of the candidate the keyboard is on: the
// candidate's own wash, made strong enough to stand out from the others on a
// sidebar face.
func currentCandidateTint(t *ui.Theme) ui.Color {
	return t.Accent.Alpha(0.22)
}
