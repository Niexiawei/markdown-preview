package tray

import (
	"bytes"
	"image/png"
	"testing"
)

// 两张图都要能解码，且都是方形：PNG 坏了或不是图标时会静默变成空图标。
func TestEmbeddedIconsDecode(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"window", WindowIcon()},
		{"tray", Icon()},
	} {
		if len(c.data) == 0 {
			t.Fatalf("%s icon: empty", c.name)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(c.data))
		if err != nil {
			t.Fatalf("%s icon: %v", c.name, err)
		}
		if cfg.Width != cfg.Height {
			t.Fatalf("%s icon: %dx%d, want square", c.name, cfg.Width, cfg.Height)
		}
	}
}

// 任务栏/窗口图标给 256（高 DPI 下也清晰），托盘给 32（16pt 的 2x）。
func TestIconSizes(t *testing.T) {
	for _, c := range []struct {
		name string
		data []byte
		want int
	}{
		{"window", WindowIcon(), 256},
		{"tray", Icon(), 32},
	} {
		cfg, err := png.DecodeConfig(bytes.NewReader(c.data))
		if err != nil {
			t.Fatalf("%s icon: %v", c.name, err)
		}
		if cfg.Width != c.want || cfg.Height != c.want {
			t.Fatalf("%s icon: %dx%d, want %dx%d", c.name, cfg.Width, cfg.Height, c.want, c.want)
		}
	}
}
