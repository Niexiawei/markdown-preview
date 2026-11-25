// Package launch reads the documents a launch asked the app to open: the
// markdown files on a command line, which the system hands over when a file
// is opened with the app.
package launch

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"mdpreview/internal/workspace"
)

// Documents returns the markdown files among args, as absolute paths in the
// order given: a relative argument is joined to wd, and a file:// URL becomes
// the path it names. Anything that is not an existing markdown file is left
// out rather than reported, since a launch may carry flags, a directory, or a
// path that went away between the launch and the window.
func Documents(args []string, wd string) []string {
	var out []string
	for _, arg := range args {
		p := arg
		if strings.HasPrefix(strings.ToLower(arg), "file://") {
			var ok bool
			if p, ok = fileURL(arg); !ok || p == "" {
				continue
			}
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(wd, p)
		}
		abs, err := filepath.Abs(p)
		if err != nil || !workspace.IsMarkdown(abs) {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		out = append(out, abs)
	}
	return out
}

// fileURL turns a file:// URL into the path it names, and reports whether it
// is one the app can open: a URL of another host names another machine.
func fileURL(arg string) (string, bool) {
	u, err := url.Parse(arg)
	if err != nil {
		return "", false
	}
	p := u.Path
	switch {
	case u.Host == "" || u.Host == "localhost":
	case len(u.Host) == 2 && u.Host[1] == ':':
		// A Windows file URL may name the drive as the host: file://C:/notes.
		p = u.Host + p
	default:
		// Another machine's file is not this one's.
		return "", false
	}
	// A Windows drive is written /C:/notes, and the leading slash is not
	// part of it.
	if len(p) > 2 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p), true
}
