package main

import (
	"log"
	"net"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/oneo-me/onevra/source/shells/sidebar"

	"mdpreview/internal/config"
	"mdpreview/internal/mcp"
	"mdpreview/internal/view"
	"mdpreview/internal/workspace"
)

func main() {
	mygo.App.WhenReady(func() {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Printf("user data directory: %v", err)
		}
		cfg, err := config.Load(dir)
		if err != nil {
			log.Printf("configuration: %v", err)
		}
		ws := workspace.New(cfg)
		state := view.New(ws, cfg, dir)
		view.RegisterMDView(state.Roots)
		// The shell owns the window, the floating header and the sidebar the
		// user collapses; the state fills its slots.
		shell := sidebar.NewShell(sidebar.ShellOptions{
			Title:        "Markdown Preview",
			Width:        1280,
			Height:       820,
			MinWidth:     760,
			MinHeight:    520,
			StateKey:     "markdown-preview.main",
			LeftWidth:    280,
			LeftMinWidth: 200,
			Left:         state.Left,
			LeftHeader:   state.LeftHeader,
			LeftToolbar:  state.LeftToolbar,
			ContentTitle: state.ContentTitle,
			Content:      state.Content,
			Theme:        view.AppTheme,
		})
		win := shell.Window()
		state.SetWindow(win)
		// The body takes the pane's width so paragraphs wrap and re-wrap on
		// resize; only the shell knows how much the sidebar takes.
		state.SetContentWidth(func() float32 {
			width, _ := win.ContentSize()
			sidebarWidth := float32(0)
			if shell.LeftExpanded() {
				sidebarWidth = shell.LeftWidth()
			}
			return float32(width) - sidebarWidth
		})
		state.StartWatching()
		startMCP(state, ws, cfg, dir)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// startMCP starts the local MCP server when the configuration asks for it,
// and gives the window the control that turns it on and off. It runs on the
// window's thread, so it may set the window's state directly.
func startMCP(state *view.State, ws *workspace.Workspace, cfg config.Config, dir string) {
	var running *mcp.Server

	start := func() {
		if running != nil {
			return
		}
		srv, err := mcp.New(mcp.Options{
			Index: ws,
			Addr:  "127.0.0.1:" + strconv.Itoa(cfg.MCP.Port),
			Token: cfg.MCP.Token,
		})
		if err != nil {
			log.Printf("mcp: %v", err)
			return
		}
		addr, err := srv.Start()
		if err != nil {
			// The port is taken: let the system choose one, and remember it.
			srv.Close()
			srv, err = mcp.New(mcp.Options{Index: ws, Addr: "127.0.0.1:0", Token: cfg.MCP.Token})
			if err != nil {
				log.Printf("mcp: %v", err)
				return
			}
			addr, err = srv.Start()
			if err != nil {
				log.Printf("mcp: %v", err)
				return
			}
		}
		running = srv
		cfg.MCP.Enabled = true
		cfg.MCP.Token = srv.Token()
		if _, port, err := net.SplitHostPort(addr); err == nil {
			if n, err := strconv.Atoi(port); err == nil {
				cfg.MCP.Port = n
			}
		}
		if err := cfg.Save(dir); err != nil {
			log.Printf("mcp: %v", err)
		}
		if err := srv.WriteClientInfo(dir); err != nil {
			log.Printf("mcp: %v", err)
		}
		state.SetMCPStatus(true, "http://"+addr+"/mcp", srv.Token())
	}

	stop := func() {
		if running == nil {
			return
		}
		if err := running.Close(); err != nil {
			log.Printf("mcp: %v", err)
		}
		running = nil
		cfg.MCP.Enabled = false
		if err := cfg.Save(dir); err != nil {
			log.Printf("mcp: %v", err)
		}
		state.SetMCPStatus(false, "", "")
	}

	state.SetMCPControl(func(on bool) {
		if on {
			start()
			return
		}
		stop()
	})
	if cfg.MCP.Enabled {
		start()
	} else {
		state.SetMCPStatus(false, "", "")
	}
}
