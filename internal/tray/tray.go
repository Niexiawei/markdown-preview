package tray

import "github.com/egoist/mygo"

// Show brings the window back from hidden or minimized and gives it the
// keyboard. Both the tray's "open" item and a second launch go through here, so
// that there is one way back no matter how the window went away.
func Show(win *mygo.Window) {
	if win == nil {
		return
	}
	win.Restore()
	win.Show()
	win.Focus()
}

// menuItems is everything the tray offers: open the window, or quit. No
// separator and no third item — the tray is the app's only way to quit.
func menuItems(win *mygo.Window) []*mygo.MenuItem {
	return []*mygo.MenuItem{
		{
			Label: "打开主页面",
			Click: func(*mygo.MenuItem, *mygo.Window) { Show(win) },
		},
		{
			Label: "退出",
			Click: func(*mygo.MenuItem, *mygo.Window) { mygo.App.Quit() },
		},
	}
}

// Install adds the tray icon and keeps the app running in it: closing the
// window hides it, the tray's menu shows it again or quits, and macOS brings it
// back when the user clicks the Dock icon with no window showing.
//
// Call it on the main thread once the app is ready, with the shell's window. It
// creates the tray before it takes over the window's close: where no tray can
// exist (a Linux desktop without a status area), the caller gets an error and
// the window keeps closing the app, rather than disappearing with no way back.
func Install(win *mygo.Window, toolTip string) error {
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "main",
		Icon:           Icon(),
		IconIsTemplate: true,
		ToolTip:        toolTip,
		Menu:           mygo.NewMenu(menuItems(win)),
	})
	if err != nil {
		return err
	}
	(&keep{win: win}).install()
	mygo.App.OnActivate(func(hasVisibleWindows bool) {
		if !hasVisibleWindows {
			Show(win)
		}
	})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { t.Destroy() })
	return nil
}
