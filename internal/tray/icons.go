// Package tray puts the app in the notification area (Windows), the menu bar
// (macOS) or the status area (Linux): its icon, its two menu items, and the
// rule that closing the window only hides it. It also carries the window's
// icon, so that the binary draws its own mark wherever it runs.
//
// It never reaches into internal/view: everything here is window and app
// lifecycle, decided once, not per frame.
package tray

import _ "embed"

// windowPNG is the window and taskbar icon (Windows, Linux; macOS windows show
// no icon of their own). It is 256×256 so that high-DPI taskbars scale down
// instead of up.
//
//go:embed assets/window.png
var windowPNG []byte

// trayPNG is the icon in the notification area: the document silhouette in
// ink, on transparency. With IconIsTemplate the desktop tints it for the bar
// it sits on (macOS menu bar, a dark Windows taskbar, a dark panel), so the
// kit's tray-on-dark variant is not shipped.
//
//go:embed assets/tray.png
var trayPNG []byte

// WindowIcon returns the PNG to give a window with SetIcon.
func WindowIcon() []byte { return windowPNG }

// Icon returns the PNG for the notification area.
func Icon() []byte { return trayPNG }
