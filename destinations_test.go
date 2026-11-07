package main

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNavigationFitsMinimumWindowWidth(t *testing.T) {
	for _, width := range []int{860, 1280, 1482} {
		a := &app{router: ui.NewRouter("/museums")}
		view := ui.NewTester(func(c *ui.Context) {
			a.theme(c)
			ui.Column(c).Fill().Children(func() { a.topBar(c) })
		}, width, 100)
		view.SetTitleBar(ui.TitleBar{Height: 56, Left: 80})
		for _, label := range []string{"Home", "Explore", "Museums", "Places", "Time machine", "Favorites", "Settings", "Search the Met collection"} {
			r, ok := view.Find(label)
			if !ok || r.X < 0 || r.X+r.W > float32(width) || r.W == 0 {
				t.Errorf("%s at width %d: bounds=%+v, found=%v", label, width, r, ok)
			}
		}
	}
}

func TestDestinationNavigationAndSourceLinks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	museum := cultureItem{Name: "Example Museum", Path: "/partner/example-museum"}
	work := cultureItem{Name: "Example painting", Path: "/asset/example-painting"}
	router := ui.NewRouter("/places")
	router.Push("/places/view?path=%2Fentity%2Ftokyo%2Fm07dfk")
	a := &app{
		router: router,
		img:    &images{m: map[picKey]*picture{}},
		directories: map[destinationKind]*directoryState{
			museums: {feed: &cultureFeed{q: cultureQuery{Kind: museums}, items: []cultureItem{museum}, total: 1, done: true, ctx: ctx, cancel: cancel}},
			places:  {feed: &cultureFeed{q: cultureQuery{Kind: places}, done: true, ctx: ctx, cancel: cancel}},
		},
		destinations: map[string]*destinationState{
			"/entity/tokyo/m07dfk": {detail: &destination{cultureItem: cultureItem{Name: "Tokyo", Path: "/entity/tokyo/m07dfk"}, Museums: []cultureItem{museum}}},
			museum.Path:            {detail: &destination{cultureItem: museum, Works: []cultureItem{work}}},
		},
	}
	view := ui.NewTester(a.view, 1280, 840)
	if err := view.Click("Example Museum"); err != nil {
		t.Fatal(err)
	}
	view.Advance(20 * time.Millisecond)
	if r, ok := view.Find("Museum & collection"); !ok || r.X < 0 || r.X+r.W > 1280 {
		t.Fatalf("detail page slid outside the content area: %+v, found=%v", r, ok)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/museums/view" || !view.HasText("Collection highlights") {
		t.Fatal("related museum did not open natively")
	}
	if err := view.Click("Open Example painting on Google Arts & Culture"); err != nil {
		t.Fatal(err)
	}
	if links := view.OpenedURLs(); len(links) != 1 || links[0] != cultureBase+work.Path {
		t.Fatalf("opened links = %v", links)
	}
	a.router.Back()
	view.Frame()
	view.Advance(time.Second)
	if a.router.Path() != "/places/view" || !view.HasText("Museums & collections in Tokyo") {
		t.Fatal("back did not return to the place")
	}
}

func TestDestinationBackReturnsToIndex(t *testing.T) {
	for _, tc := range []struct {
		kind destinationKind
		item cultureItem
	}{
		{museums, cultureItem{Name: "Tokyo Museum", Path: "/partner/tokyo-museum"}},
		{places, cultureItem{Name: "Tokyo", Path: "/entity/tokyo/m07dfk"}},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &cultureFeed{q: cultureQuery{Kind: tc.kind, Text: "Tokyo"}, items: []cultureItem{tc.item}, total: 1, done: true, ctx: ctx, cancel: cancel}
			s := &directoryState{query: "Tokyo", feed: f}
			index := "/" + string(tc.kind)
			a := &app{
				router:       ui.NewRouter(index),
				img:          &images{m: map[picKey]*picture{}},
				directories:  map[destinationKind]*directoryState{tc.kind: s},
				destinations: map[string]*destinationState{tc.item.Path: {detail: &destination{cultureItem: tc.item}}},
			}
			view := ui.NewTester(a.view, 1280, 840)
			if err := view.Click(tc.item.Name); err != nil {
				t.Fatal(err)
			}
			view.Advance(time.Second)
			if a.router.Path() != index+"/view" || !a.router.CanGoBack() {
				t.Fatal("opening the detail lost the index history entry")
			}
			if err := view.Click("Back"); err != nil {
				t.Fatal(err)
			}
			view.Advance(time.Second)
			if a.router.Path() != index || a.router.CanGoBack() || !a.router.CanGoForward() {
				t.Fatal("Back did not return to the index")
			}
			if s.query != "Tokyo" || s.feed != f || !view.HasText("matching “Tokyo”") {
				t.Fatal("Back lost the index search or loaded results")
			}
			if err := view.Click("Forward"); err != nil {
				t.Fatal(err)
			}
			view.Advance(time.Second)
			if a.router.Path() != index+"/view" {
				t.Fatal("Forward did not reopen the detail")
			}
		})
	}
}
