package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:  "Markdown Preview",
			Width:  1280,
			Height: 820,
			Content: ui.View(func(c *ui.Context) {
				ui.Text(c, "Markdown Preview")
			}),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
