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
		for _, label := range []string{"Home", "Artists", "Museums", "Places", "Time machine", "Favorites", "Settings", "Search artworks or open a link"} {
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
	story := cultureItem{Name: "Example story", Path: "/story/example-story"}
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
			museum.Path:            {detail: &destination{cultureItem: museum, Works: []cultureItem{work}, Stories: []cultureItem{story}}},
		},
		assets: map[string]*assetState{"example-painting": {art: &cultureArtwork{ID: "example-painting", Title: "Example painting", Path: work.Path}}},
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
	if err := view.Click("Example painting"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/asset/example-painting" || len(view.OpenedURLs()) != 0 {
		t.Fatal("artwork did not open inside the app")
	}
	if err := view.Click("Back"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	view.Advance(time.Second)
	if a.router.Path() != "/museums/view" {
		t.Fatal("Back did not return to the museum")
	}
	view.Scroll(640, 500, 0, 1000)
	if err := view.Click("Open Example story on Google Arts & Culture"); err != nil {
		t.Fatal(err)
	}
	if links := view.OpenedURLs(); len(links) != 1 || links[0] != cultureBase+story.Path || a.router.Path() != "/museums/view" {
		t.Fatalf("story links = %v, path = %s", links, a.router.Path())
	}
	if err := view.Click("Back"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/places/view" || !view.HasText("Museums & collections in Tokyo") {
		t.Fatal("back did not return to the place")
	}
}

func TestDestinationBackReturnsToIndex(t *testing.T) {
	for _, tc := range []struct {
		kind  destinationKind
		item  cultureItem
		query string
	}{
		{artists, cultureItem{Name: "Claude Monet", Path: "/entity/claude-monet/m01xnj?categoryId=artist"}, "Monet"},
		{museums, cultureItem{Name: "Tokyo Museum", Path: "/partner/tokyo-museum"}, "Tokyo"},
		{places, cultureItem{Name: "Tokyo", Path: "/entity/tokyo/m07dfk"}, "Tokyo"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &cultureFeed{q: cultureQuery{Kind: tc.kind, Text: tc.query}, items: []cultureItem{tc.item}, total: 1, done: true, ctx: ctx, cancel: cancel}
			s := &directoryState{query: tc.query, feed: f}
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
			if s.query != tc.query || s.feed != f || !view.HasText("matching “"+tc.query+"”") {
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

func TestArtistsReplaceExploreAndOpenArtworksNatively(t *testing.T) {
	artist := cultureItem{Name: "Claude Monet", Path: "/entity/claude-monet/m01xnj?categoryId=artist"}
	work := cultureItem{Name: "Water Lilies", Path: "/asset/water-lilies/example-painting"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &app{
		router: ui.NewRouter("/museums"),
		img:    &images{m: map[picKey]*picture{}},
		directories: map[destinationKind]*directoryState{
			museums: {feed: &cultureFeed{q: cultureQuery{Kind: museums}, done: true, ctx: ctx, cancel: cancel}},
			artists: {feed: &cultureFeed{q: cultureQuery{Kind: artists}, items: []cultureItem{artist}, total: 1, done: true, ctx: ctx, cancel: cancel}},
		},
		destinations: map[string]*destinationState{
			artist.Path: {detail: &destination{cultureItem: cultureItem{Name: artist.Name, Path: artist.Path, Subtitle: "1840–1926"}, Description: "French impressionist painter.", Works: []cultureItem{work}}},
		},
		assets: map[string]*assetState{"example-painting": {art: &cultureArtwork{ID: "example-painting", Title: work.Name, Path: work.Path}}},
	}
	view := ui.NewTester(a.view, 1280, 840)
	if _, ok := view.Find("Explore"); ok {
		t.Fatal("Explore is still in the navigation")
	}
	if err := view.Click("Artists"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/artists" || !view.HasText("1 artist") {
		t.Fatal("Artists did not open the Google Arts & Culture directory")
	}
	if err := view.Click(artist.Name); err != nil {
		t.Fatal(err)
	}
	view.Advance(20 * time.Millisecond)
	if r, ok := view.Find("Artist"); !ok || r.X < 0 || r.X+r.W > 1280 {
		t.Fatalf("artist details moved outside the content area: %+v, found=%v", r, ok)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/artists/view" || !view.HasText("Selected works") || !view.HasText("1840–1926") {
		t.Fatal("artist details did not open natively with dates and works")
	}
	if err := view.Click(work.Name); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/asset/example-painting" || len(view.OpenedURLs()) != 0 {
		t.Fatal("artist artwork did not open in the shared viewer")
	}
	if err := view.Click("Back"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/artists/view" || !view.HasText(artist.Name) {
		t.Fatal("Back did not return to the artist")
	}
}
