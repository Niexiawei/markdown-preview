package tray

import (
	"testing"

	"github.com/egoist/mygo"
)

// 托盘菜单只有两项，顺序固定：先打开主页面，再退出。不加分隔线、不加第三项。
func TestTrayMenuOpensTheWindowOrQuits(t *testing.T) {
	items := mygo.NewMenu(menuItems(nil)).Items()
	if len(items) != 2 {
		t.Fatalf("tray menu has %d items, want 2", len(items))
	}
	for i, want := range []string{"打开主页面", "退出"} {
		it := items[i]
		if it.Label != want {
			t.Fatalf("item %d label = %q, want %q", i, it.Label, want)
		}
		if it.Type == mygo.MenuItemSeparator {
			t.Fatalf("item %d (%s) is a separator", i, want)
		}
		if it.Click == nil {
			t.Fatalf("item %d (%s) does nothing", i, want)
		}
	}
}
