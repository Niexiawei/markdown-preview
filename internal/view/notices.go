package view

import (
	"time"

	"github.com/oneo-me/onevra/source/components"

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
