package main

import (
	"context"
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
)

type assetState struct {
	seed    cultureItem
	art     *cultureArtwork
	viewer  viewer
	loading bool
	err     error
}

func (a *app) openCultureArtwork(item cultureItem) {
	id := cultureAssetID(item.Path)
	if id == "" {
		return
	}
	if a.assets == nil {
		a.assets = map[string]*assetState{}
	}
	s := a.assets[id]
	if s == nil {
		s = &assetState{}
		a.assets[id] = s
	}
	// A card knows more than a pasted link, so its fields replace older ones.
	if item.Name != "" {
		s.seed.Name = item.Name
	}
	if item.Subtitle != "" {
		s.seed.Subtitle = item.Subtitle
	}
	if item.Image != "" {
		s.seed.Image = item.Image
	}
	// A shared link's region of the picture shows once it has loaded.
	if p, ok := cultureAssetViewport(item.Path); ok {
		s.viewer.pending, s.viewer.didShow = &p, false
	}
	// Opening the artwork again retries details that failed before, and a
	// pyramid that could not be reached; one that does not exist stays off.
	if s.art == nil && !s.loading {
		s.err = nil
	}
	if t := s.viewer.tiles; t != nil && t.err != nil && !errors.Is(t.err, errNoTiles) {
		t.err = nil
	}
	a.router.Push("/asset/" + id)
}

func (a *app) loadCultureArtwork(id string, s *assetState) {
	s.loading, s.err = true, nil
	go func() {
		art, err := a.culture.Artwork(context.Background(), id)
		a.update(func() { s.art, s.err, s.loading = art, err, false })
	}()
}

func (a *app) assetPage(c *ui.Context, id string) {
	if a.assets == nil {
		a.assets = map[string]*assetState{}
	}
	s := a.assets[id]
	if s == nil {
		s = &assetState{}
		a.assets[id] = s
	}
	if s.art == nil && !s.loading && s.err == nil {
		a.loadCultureArtwork(id, s)
	}
	a.prepareTiles(id, &s.viewer)
	d := s.art
	preview, full := s.seed.Image, ""
	if d == nil {
		d = &cultureArtwork{ID: id, Title: s.seed.Name, Artist: s.seed.Subtitle}
	} else {
		preview, full = cultureImage(d.Image, 1800), cultureImage(d.Image, 4096)
		// Keep the card's picture on screen until the larger one has arrived.
		if p := a.img.get(preview, 1800); s.seed.Image != "" && (p == nil || p.bmp == nil) {
			preview = s.seed.Image
		}
	}
	title := d.Title
	if title == "" {
		title = "Artwork"
	}
	ui.Row(c.Key(id)).Fill().AlignItems(ui.Stretch).Children(func() {
		if s.loading && preview == "" {
			ui.Box(c).Grow(1).Background(night).Center().Children(func() { ui.Spinner(c).FontSize(22).TextColor(inkFaint) })
		} else {
			a.imageViewerPane(c, title, []viewerImage{{preview: preview, full: full}}, &s.viewer, "Loading larger image")
		}
		ui.Scroll(c.Key("asset-details")).Hidden(a.immersive).Width(420).Shrink(0).Background(paper).BorderWidth(0, 0, 0, 1).BorderColor(line).Children(func() {
			ui.Column(c).Padding(32, 30, 40).Gap(22).Children(func() {
				ui.Column(c).Gap(10).Children(func() {
					ui.Text(c, "Google Arts & Culture").FontSize(12).TextColor(accent)
					ui.Text(c, title).Font(serif).FontSize(30).LineHeight(1.15).Selectable().Pretty()
					if d.Artist != "" {
						ui.Text(c, d.Artist).FontSize(16).FontWeight(500).Selectable()
					}
					if d.Date != "" {
						ui.Text(c, d.Date).TextColor(inkSoft).Selectable()
					}
				})
				if s.loading {
					ui.Spinner(c).FontSize(18).AlignSelf(ui.Start)
				}
				if s.err != nil {
					ui.Text(c, "Artwork details could not be loaded. Check your connection and try again.").TextColor(inkSoft)
					if ui.Button(c.Key("retry-asset"), "Try again").Clicked() {
						a.loadCultureArtwork(id, s)
					}
				}
				if d.Museum.Path != "" {
					b := ui.ButtonBase(c.Key("asset-museum")).Column().AlignItems(ui.Start).Gap(5).Padding(16).Radius(8).Background(wash).Cursor(ui.CursorPointer).Label("View museum: " + d.Museum.Name)
					b.Children(func() {
						ui.Text(c, d.Museum.Name).Font(serif).FontSize(20)
						if d.Museum.Subtitle != "" {
							ui.Text(c, d.Museum.Subtitle).FontSize(12).TextColor(inkSoft)
						}
						ui.Text(c, "View museum").FontSize(12).TextColor(accent)
					})
					if b.Clicked() {
						a.openDestination(d.Museum)
					}
				}
				if d.Description != "" {
					ui.Column(c).Gap(12).Children(func() {
						ui.Text(c, "About this artwork").Font(serif).FontSize(20)
						for i, paragraph := range strings.Split(d.Description, "\n\n") {
							ui.Text(c.Key(i), paragraph).FontSize(14).LineHeight(1.5).TextColor(inkSoft).Selectable()
						}
					})
				}
				if len(d.Details) > 0 {
					ui.Divider(c)
					ui.Text(c, "Details").Font(serif).FontSize(20)
					ui.Grid(c).ColumnTracks(ui.Fixed(110), ui.Fr(1)).GapX(14).GapY(12).Children(func() {
						for i, detail := range d.Details {
							ui.Text(c.Key(2*i), detail.Label).FontSize(12).TextColor(inkFaint)
							if detail.URL != "" {
								externalButton(c.Key(2*i+1), detail.Value, detail.URL)
							} else {
								ui.Text(c.Key(2*i+1), detail.Value).FontSize(13).Selectable()
							}
						}
					})
				}
				if d.Path != "" {
					externalButton(c, "View on Google Arts & Culture", cultureBase+d.Path)
				}
			})
		})
	})
}
