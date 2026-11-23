package tray

import "github.com/egoist/mygo"

// keep is the rule that closing the window does not end the app: OnClose hides
// the window instead, and only a real quit lets the close through.
//
// All fields are read and written on the main thread, where App's and the
// window's listeners run.
type keep struct {
	win      *mygo.Window
	quitting bool
}

// install hooks the window up. The quit flag comes from App.OnBeforeQuit,
// which runs before App.Quit closes every window: without it, Quit would be
// blocked by close and the app could never exit.
func (k *keep) install() {
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { k.quitting = true })
	if k.win != nil {
		k.win.OnClose(k.close)
	}
}

// close is the window's OnClose: hide the window, unless the app is quitting.
func (k *keep) close(e *mygo.CloseEvent) {
	if k.quitting {
		return
	}
	e.PreventDefault()
	if k.win != nil {
		k.win.Hide()
	}
}
