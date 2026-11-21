package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mdpreview/internal/config"
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
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:   "Markdown Preview",
			Width:   1280,
			Height:  820,
			Content: ui.View(state.View),
		})
		state.SetWindow(win)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
