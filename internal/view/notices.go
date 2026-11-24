package view

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/oneo-me/onevra/source/components"
	"github.com/oneo-me/onevra/source/icons/lucide"

	"mdpreview/internal/notices"
)

// The window's messages: what it reported to the user, as the toasts pop up and
// as the bell's history keeps them. The book of them is internal/notices, which
// saves them beside the configuration; the view only reads it, and the pieces
// the window builds from it are here.

// notify records what the window did, as a message that says so.
func (s *State) notify(text string) { s.report(components.ToastInfo, text) }

// notifyDone records something that went through.
func (s *State) notifyDone(text string) { s.report(components.ToastSuccess, text) }

// notifyWarn records something the user should know about, which is not a
// failure of the window's own.
func (s *State) notifyWarn(text string) { s.report(components.ToastWarning, text) }

// notifyErr records something that did not work.
func (s *State) notifyErr(text string) { s.report(components.ToastError, text) }

// report records one message of kind, saying text. It runs on the window's
// thread, as the actions that report do. A message that cannot be saved stays
// in the history of this run: saving is what survives a restart, not what
// shows.
func (s *State) report(kind, text string) {
	if text == "" {
		return
	}
	_, _ = s.notes.Add(kind, text, time.Now())
}

// Messages returns the messages the window reported, the newest first.
func (s *State) Messages() []notices.Message { return s.notes.Messages() }

// RemoveMessage forgets one message, as the history's own ✕ does.
func (s *State) RemoveMessage(id int) { _ = s.notes.Remove(id) }

// ClearMessages forgets every message, as the history's clear button does.
func (s *State) ClearMessages() { _ = s.notes.Clear() }

// noticeToastText is how much of a message its toast shows: the history keeps
// the whole of it, the toast is a line of the window.
const noticeToastText = 120

// updateToasts hands the messages the window has not toasted yet to this
// frame's toasts, the oldest first, so that the newest of them shows last. A
// click builds the view twice, and only the second build's asks survive it; the
// ID of the last message toasted is what keeps the second build from adding the
// same toast again. It runs in the build pass, before the shell draws them.
func (s *State) updateToasts(c *ui.Context) {
	msgs := s.Messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.ID <= s.noticeShown {
			continue
		}
		c.AddToast(ui.Toast{
			ID:    "notice-" + strconv.Itoa(m.ID),
			Title: trimEllipsis(m.Text, noticeToastText),
			Type:  m.Kind,
		})
	}
	if len(msgs) > 0 && msgs[0].ID > s.noticeShown {
		s.noticeShown = msgs[0].ID
	}
}

// The history the bell opens, and the badge over it.
const (
	// noticePanelWidth and noticePanelHeight size the panel the history shows
	// in, and the room its list scrolls within.
	noticePanelWidth  float32 = 360
	noticePanelHeight float32 = 320
	// noticeBadgeMax is the most the badge counts: past it, it says "99+",
	// which stays a pill of two digits wide.
	noticeBadgeMax = 99
)

// bellButton builds the sidebar's bell with the badge of what the user has not
// read, and the panel of the history behind it.
func (s *State) bellButton(c *ui.Context) {
	t := c.Theme()
	unread := s.notes.Unseen()
	label := "消息"
	if unread > 0 {
		label = "消息（" + strconv.Itoa(unread) + " 条未读）"
	}
	size := t.Space(8)
	var button ui.Element
	// The badge hangs over the button's corner, so it needs a box of the
	// button's own size to be placed against.
	ui.Box(c).Width(size).Height(size).Children(func() {
		button = components.IconButton(c, lucide.Bell(), label, components.IconButtonPlain).Size(size, size)
		if unread > 0 {
			button.TextColor(t.Accent)
		}
		s.bellBadge(c, unread)
	})
	if button.Clicked() {
		s.noticeOpen = !s.noticeOpen
		if s.noticeOpen {
			// Opening the history is reading it: the badge starts over.
			_ = s.notes.MarkSeen()
		}
	}
	ui.Popover(c, button, &s.noticeOpen, func() { s.noticePanel(c) })
}

// bellBadge builds the red count over the bell, or nothing when every message
// has been read. It takes no input: a press on it belongs to the bell it sits
// on.
func (s *State) bellBadge(c *ui.Context, unread int) ui.Element {
	text := noticeBadgeText(unread)
	if text == "" {
		return ui.Element{}
	}
	t := c.Theme()
	size := t.Space(3.5)
	return components.Badge(c, text, components.BadgeOptions{Tone: components.BadgeDanger}).
		Background(t.Danger).TextColor(t.AccentText).Border(1, t.Danger).
		Padding(0, t.Space(1)).MinWidth(size).Justify(ui.Center).FontSize(t.Rem(.65)).
		Absolute().Top(-2).Right(-2).IgnoreInput(true).AccessHidden(true)
}

// noticeBadgeText is what the badge says for unread messages: the count up to
// noticeBadgeMax, past it "99+", and nothing at none.
func noticeBadgeText(unread int) string {
	switch {
	case unread <= 0:
		return ""
	case unread > noticeBadgeMax:
		return strconv.Itoa(noticeBadgeMax) + "+"
	}
	return strconv.Itoa(unread)
}

// noticePanel builds the history: the count of what it holds, a button that
// clears it, and a row per message, the newest first. A message removed as the
// panel builds is not visited: the rows are made from a copy of the book, which
// removing one per frame would otherwise shorten underneath the loop.
func (s *State) noticePanel(c *ui.Context) {
	t := c.Theme()
	msgs := append([]notices.Message(nil), s.Messages()...)
	ui.Column(c).Width(noticePanelWidth).Padding(t.Space(1.5)).Gap(t.Space(1)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(t.Space(2)).Children(func() {
			components.Text(c, "消息").FontSize(13).Bold().Grow(1)
			if len(msgs) == 0 {
				return
			}
			if ui.Button(c, "清空全部").Clicked() {
				s.ClearMessages()
			}
		})
		ui.Divider(c)
		if len(msgs) == 0 {
			components.Text(c, "暂无消息").FontSize(12).TextColor(t.TextMuted)
			return
		}
		ui.Scroll(c).MaxHeight(noticePanelHeight).Children(func() {
			ui.Column(c).Gap(t.Space(0.5)).Children(func() {
				for _, m := range msgs {
					s.noticeRow(c, m)
				}
			})
		})
	})
}

// noticeRow builds one message of the history: what kind it is, what it says,
// when the window said it, and its own ✕.
func (s *State) noticeRow(c *ui.Context, m notices.Message) {
	t := c.Theme()
	ui.Row(c).AlignItems(ui.Center).Gap(t.Space(1)).Children(func() {
		components.Icon(c, noticeIcon(m.Kind)).Size(t.Rem(.875), t.Rem(.875)).TextColor(noticeTint(t, m.Kind))
		ui.Column(c).Grow(1).MinWidth(0).Gap(t.Space(.25)).Children(func() {
			components.Text(c, m.Text).FontSize(12)
			components.Text(c, noticeTime(m.At, time.Now())).FontSize(11).TextColor(t.TextMuted)
		})
		label := "删除消息：" + trimEllipsis(m.Text, 12)
		if components.IconButton(c, lucide.X(), label, components.IconButtonCompact).Clicked() {
			s.RemoveMessage(m.ID)
		}
	})
}

// noticeTime is when a message was reported, as the history reads it: the time
// alone for today, with the day before it for this year, and the whole date for
// anything older.
func noticeTime(at, now time.Time) string {
	ay, am, ad := at.Date()
	ny, nm, nd := now.Date()
	switch {
	case ay == ny && am == nm && ad == nd:
		return at.Format("15:04")
	case ay == ny:
		return at.Format("01-02 15:04")
	}
	return at.Format("2006-01-02 15:04")
}

// noticeIcon is the mark of what a message reports, beside it in the history.
func noticeIcon(kind string) *ui.SVG {
	switch kind {
	case components.ToastSuccess:
		return lucide.CircleCheck()
	case components.ToastWarning:
		return lucide.TriangleAlert()
	case components.ToastError:
		return lucide.CircleX()
	}
	return lucide.Info()
}

// noticeTint colors that mark, as the toast of the same kind colors its own.
func noticeTint(t *ui.Theme, kind string) ui.Color {
	switch kind {
	case components.ToastSuccess:
		return t.Success
	case components.ToastWarning:
		return t.Warning
	case components.ToastError:
		return t.Danger
	}
	return t.Accent
}

// ToastOptions is where the window shows a message in the Onevra look: the
// bottom right corner, where the old status bar ran out, out of the way of the
// body's first lines.
func ToastOptions() components.ToastOptions {
	return components.ToastOptions{Placement: components.ToastBottomEnd}
}
