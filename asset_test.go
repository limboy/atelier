package main

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestNativeCultureArtworkViewer(t *testing.T) {
	id := "GgHsT2RumWxbtw"
	museum := cultureItem{Name: "Musée d’Orsay, Paris", Path: "/partner/musee-dorsay-paris"}
	s := &assetState{art: &cultureArtwork{
		ID: id, Title: "Gleaners", Artist: "Jean-François Millet", Date: "1857", Path: "/asset/gleaners/" + id,
		Description: "An artwork description.", Museum: museum, Details: []cultureProperty{{Label: "Rights", Value: "© Musée d'Orsay"}},
	}}
	a := &app{router: ui.NewRouter("/asset/" + id), img: &images{m: map[picKey]*picture{}},
		assets: map[string]*assetState{id: s}, destinations: map[string]*destinationState{museum.Path: {detail: &destination{cultureItem: museum}}},
	}
	view := ui.NewTester(a.view, 1280, 840)
	if !view.HasText("Gleaners") || !view.HasText("© Musée d'Orsay") || !view.HasText("About this artwork") {
		t.Fatal("native artwork metadata missing")
	}
	if err := view.Click("Zoom in"); err != nil {
		t.Fatal(err)
	}
	if s.viewer.zoom <= 1 {
		t.Fatal("zoom control did not zoom")
	}
	if err := view.Click("Fit to window"); err != nil {
		t.Fatal(err)
	}
	if s.viewer.zoom != 1 {
		t.Fatal("fit did not reset zoom")
	}
	view.Key(ui.Cmd, ui.KeyEqual)
	if s.viewer.zoom <= 1 {
		t.Fatal("keyboard zoom did not work")
	}
	view.Key(ui.Cmd, ui.Key0)
	if err := view.Click("Full screen"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	if !a.immersive {
		t.Fatal("artwork left immersive mode")
	}
	if err := view.Click("Exit full screen"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	if a.immersive {
		t.Fatal("could not exit immersive mode")
	}
	if err := view.Click("View museum: " + museum.Name); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/museums/view" {
		t.Fatal("owning museum did not open natively")
	}
	if err := view.Click("Back"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if a.router.Path() != "/asset/"+id {
		t.Fatal("Back did not return to the artwork")
	}
	view.Scroll(1200, 700, 0, 1000)
	if err := view.Click("View on Google Arts & Culture, opens in browser"); err != nil {
		t.Fatal(err)
	}
	if links := view.OpenedURLs(); len(links) != 1 || links[0] != cultureBase+s.art.Path {
		t.Fatalf("source links = %v", links)
	}
}

func TestArtworkWithoutPicture(t *testing.T) {
	id := "GgHsT2RumWxbtw"
	a := &app{router: ui.NewRouter("/asset/" + id), img: &images{m: map[picKey]*picture{}},
		assets: map[string]*assetState{id: {art: &cultureArtwork{ID: id, Title: "Gleaners"}}},
	}
	view := ui.NewTester(a.view, 1280, 840)
	if !view.HasText("No picture is available for this artwork.") || view.HasText("Retry image") {
		t.Fatal("a missing picture looked like a failed download")
	}
}

func TestPastedCultureArtworkLinkOpensNatively(t *testing.T) {
	a := &app{router: ui.NewRouter("/museums"), query: "https://artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw"}
	view := ui.NewTester(func(c *ui.Context) {
		a.theme(c)
		ui.Column(c).Fill().Children(func() { a.topBar(c) })
	}, 1280, 100)
	if err := view.Click("Search artworks or open a link"); err != nil {
		t.Fatal(err)
	}
	view.Key(0, ui.KeyEnter)
	if a.router.Path() != "/asset/GgHsT2RumWxbtw" || !a.router.CanGoBack() || len(view.OpenedURLs()) != 0 {
		t.Fatal("pasted artwork link did not open natively with history")
	}
}

func TestEscapeLeavesFullScreenThenArtwork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id := "GgHsT2RumWxbtw"
	router := ui.NewRouter("/museums")
	router.Push("/asset/" + id)
	a := &app{router: router, img: &images{m: map[picKey]*picture{}},
		directories: map[destinationKind]*directoryState{
			museums: {feed: &cultureFeed{q: cultureQuery{Kind: museums}, done: true, ctx: ctx, cancel: cancel}},
		},
		assets: map[string]*assetState{id: {art: &cultureArtwork{ID: id, Title: "Gleaners"}}},
	}
	view := ui.NewTester(a.view, 1280, 840)

	// A focused search field keeps Esc for itself.
	if err := view.Click("Search artworks or open a link"); err != nil {
		t.Fatal(err)
	}
	view.Key(0, ui.KeyEscape)
	view.Frame()
	if a.router.Path() != "/asset/"+id {
		t.Fatal("Esc in the search field left the artwork")
	}
	if err := view.Click("Gleaners"); err != nil {
		t.Fatal(err)
	}

	view.Key(0, ui.KeyF)
	view.Frame()
	if !a.immersive {
		t.Fatal("F did not enter full screen")
	}
	view.Key(0, ui.KeyEscape)
	view.Frame()
	if a.immersive || a.router.Path() != "/asset/"+id {
		t.Fatalf("first Esc: immersive=%v path=%s, want the artwork without full screen", a.immersive, a.router.Path())
	}
	view.Key(0, ui.KeyEscape)
	view.Advance(time.Second)
	if a.router.Path() != "/museums" {
		t.Fatalf("second Esc: path=%s, want /museums", a.router.Path())
	}
}

func TestEscapeWithoutHistoryGoesHome(t *testing.T) {
	id := "GgHsT2RumWxbtw"
	a := &app{router: ui.NewRouter("/asset/" + id), img: &images{m: map[picKey]*picture{}},
		assets: map[string]*assetState{id: {art: &cultureArtwork{ID: id, Title: "Gleaners"}}},
	}
	view := ui.NewTester(func(c *ui.Context) {
		if a.router.Path() == "/asset/"+id {
			a.assetPage(c, id)
		}
	}, 1280, 840)
	view.Key(0, ui.KeyEscape)
	view.Frame()
	if a.router.Path() != "/home" {
		t.Fatalf("path=%s, want /home", a.router.Path())
	}
}
