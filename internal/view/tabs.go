package view

import "github.com/egoist/mygo/ui"

// openTab shows the document sel, adding it to the tabs unless it is open
// there already. Every way of showing a document — the tree, a link in it, a
// search hit — comes through here, so that one document is one tab.
func (s *State) openTab(sel docSel) {
	if sel.empty() || sel == s.sel {
		return
	}
	if !s.hasTab(sel) {
		s.tabs = append(s.tabs, sel)
	}
	// Every tab keeps where its body was scrolled, so that switching back
	// shows the document where the user left it.
	if _, ok := s.scrolls[sel]; !ok {
		s.scrolls[sel] = &ui.ScrollState{}
	}
	s.activateTab(sel)
}

// activateTab makes sel the document shown, without adding a tab.
func (s *State) activateTab(sel docSel) {
	s.sel = sel
	s.status = ""
	// Another document's outline starts at its top, not where the last
	// one's scroll was left. A reload of the same document keeps it.
	s.tocScroll.X, s.tocScroll.Y = 0, 0
	s.onDocChanged()
}

// hasTab reports whether the document is open in a tab.
func (s *State) hasTab(sel docSel) bool { return s.tabIndex(sel) >= 0 }

// tabIndex is where the document's tab sits, or -1.
func (s *State) tabIndex(sel docSel) int {
	for i, tab := range s.tabs {
		if tab == sel {
			return i
		}
	}
	return -1
}

// closeTab closes the document's tab. Closing the document shown shows the
// tab to its right, else the one to its left, else nothing.
func (s *State) closeTab(sel docSel) {
	i := s.tabIndex(sel)
	if i < 0 {
		return
	}
	s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	delete(s.scrolls, sel)
	if sel != s.sel {
		return
	}
	switch {
	case i < len(s.tabs):
		s.activateTab(s.tabs[i])
	case i > 0:
		s.activateTab(s.tabs[i-1])
	default:
		s.sel = docSel{}
		s.status = ""
		s.onDocChanged()
	}
}

// dropTab forgets the document's tab without changing what the window shows:
// the caller is already clearing it, as when the file went away.
func (s *State) dropTab(sel docSel) {
	if i := s.tabIndex(sel); i >= 0 {
		s.tabs = append(s.tabs[:i], s.tabs[i+1:]...)
	}
	delete(s.scrolls, sel)
}

// dropTabsOf closes every tab of one project: a project that is closed or
// removed takes its documents with it.
func (s *State) dropTabsOf(projectID string) {
	kept := s.tabs[:0]
	for _, tab := range s.tabs {
		if tab.project == projectID {
			delete(s.scrolls, tab)
			continue
		}
		kept = append(kept, tab)
	}
	s.tabs = kept
	if s.sel.project == projectID {
		s.sel = docSel{}
		s.status = ""
		s.onDocChanged()
	}
}

// renameTab moves a tab and its scroll position to its new path, keeping where
// it sat in the bar.
func (s *State) renameTab(from, to docSel) {
	i := s.tabIndex(from)
	if i < 0 {
		return
	}
	s.tabs[i] = to
	if st, ok := s.scrolls[from]; ok {
		delete(s.scrolls, from)
		s.scrolls[to] = st
	}
}
