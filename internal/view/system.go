package view

import "github.com/egoist/mygo"

// System is what the window asks of the desktop beyond drawing it: revealing a
// project in the file manager, copying a path, and asking before a project is
// removed. main supplies Desktop; the tests supply a fake.
type System interface {
	// Reveal shows a directory in the file manager.
	Reveal(path string) error
	// CopyText puts text in the clipboard.
	CopyText(text string)
	// Confirm asks the user to allow what cannot be undone in the window.
	Confirm(options ConfirmOptions) bool
}

// ConfirmOptions is what to ask, and what the button that goes ahead says.
type ConfirmOptions struct {
	Title, Message, ConfirmLabel string
}

// Desktop is the System the app runs with: the system's file manager, the
// clipboard, and a modal message box. The message box is native rather than
// one of the theme's dialogs because the sidebar header that opens it is not
// built while the sidebar is collapsed, so an in-window dialog could not draw.
type Desktop struct{}

// Reveal shows a directory in the file manager.
func (Desktop) Reveal(path string) error { return mygo.Shell.OpenPath(path) }

// CopyText puts text in the clipboard.
func (Desktop) CopyText(text string) { mygo.Clipboard.WriteText(text) }

// Confirm asks before an action the window cannot undo. The cancel button
// holds the default and Escape; the confirm button is set apart as destructive.
func (Desktop) Confirm(options ConfirmOptions) bool {
	label := options.ConfirmLabel
	if label == "" {
		label = "确定"
	}
	result, err := mygo.Dialog.Message(mygo.MessageOptions{
		Type:               mygo.MessageQuestion,
		Title:              options.Title,
		Message:            options.Message,
		Buttons:            []string{"取消", label},
		DefaultButton:      0,
		CancelButton:       0,
		DestructiveButtons: []int{1},
	})
	return err == nil && result.Button == 1
}
