package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type directoryState struct {
	query   string
	alpha   bool
	letter  string
	changed time.Time
	feed    *cultureFeed
}

type cultureFeed struct {
	q             cultureQuery
	items         []cultureItem
	total         int
	next          string
	loading, done bool
	err           error
	ctx           context.Context
	cancel        context.CancelFunc
}

type destinationState struct {
	seed              cultureItem
	detail            *destination
	loading, expanded bool
	err               error
}

func (a *app) moreDestinations(f *cultureFeed) {
	if f.loading || f.done {
		return
	}
	f.loading, f.err = true, nil
	cursor := f.next
	go func() {
		page, err := a.culture.Directory(f.ctx, f.q, cursor)
		if f.ctx.Err() != nil {
			return
		}
		a.update(func() {
			f.loading, f.err = false, err
			if err != nil {
				return
			}
			seen := map[string]bool{}
			for _, item := range f.items {
				seen[item.Path] = true
			}
			for _, item := range page.Items {
				if !seen[item.Path] {
					f.items = append(f.items, item)
					seen[item.Path] = true
				}
			}
			f.total, f.next = page.Total, page.Next
			f.done = page.Next == "" || page.Next == cursor || len(page.Items) == 0
		})
	}()
}

func (a *app) directory(c *ui.Context, kind destinationKind) {
	s := a.directories[kind]
	if s == nil {
		s = &directoryState{letter: "A"}
		a.directories[kind] = s
	}
	title, sub, placeholder := "Museums", "Visit museums, galleries and cultural collections around the world.", "Search museums or cities…"
	if kind == artists {
		title, sub, placeholder = "Artists", "Discover artists and their work from collections around the world.", "Search artists by name…"
	} else if kind == places {
		title, sub, placeholder = "Places", "Explore the art and culture of countries, cities and landmarks.", "Search cities, countries or landmarks…"
	}
	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Padding(30, 40, 20).Gap(20).Children(func() {
			ui.Row(c).AlignItems(ui.End).Gap(24).Children(func() {
				ui.Column(c).Grow(1).Gap(6).Children(func() {
					ui.Text(c, title).Font(serif).FontSize(40)
					ui.Text(c, sub).FontSize(14).TextColor(inkSoft)
				})
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				chip(c.Key("featured"), "Featured", !s.alpha, func() { s.alpha = false })
				chip(c.Key("alphabetical"), "A–Z", s.alpha, func() { s.alpha = true })
				ui.Spacer(c)
				field := ui.SearchField(c.Key("directory-search"), &s.query).Placeholder(placeholder).Label("Search " + strings.ToLower(title)).Width(340).Radius(18)
				if field.Changed() {
					s.changed = c.Now()
				}
				if c.Shortcut(ui.Cmd, ui.KeyF) {
					field.Focus()
				}
			})
			if s.alpha && strings.TrimSpace(s.query) == "" {
				ui.ScrollHorizontal(c.Key("alphabet")).Scrollbars(false).Gap(3).Children(func() {
					for _, letter := range strings.Split("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "") {
						letterButton(c.Key(letter), letter, s.letter == letter, func() { s.letter = letter })
					}
					letterButton(c.Key("other"), "#", s.letter == "-", func() { s.letter = "-" })
				})
			}
		})
		q := cultureQuery{Kind: kind, Text: strings.TrimSpace(s.query)}
		if s.alpha && q.Text == "" {
			q.Prefix = s.letter
		}
		if s.feed == nil || s.feed.q != q {
			if s.feed != nil {
				s.feed.cancel()
				s.feed = nil
			}
			if elapsed := c.Now().Sub(s.changed); q.Text != "" && elapsed < 300*time.Millisecond {
				c.After(300*time.Millisecond - elapsed)
			} else {
				ctx, cancel := context.WithCancel(context.Background())
				s.feed = &cultureFeed{q: q, ctx: ctx, cancel: cancel}
				a.moreDestinations(s.feed)
			}
		}
		f := s.feed
		// Changing a filter starts at the top; opening a detail and coming
		// back retains the directory's scroll position and loaded pages.
		ui.Scroll(c.Key(fmt.Sprint(q))).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 40, 40).Gap(22).Children(func() {
				if f == nil || f.loading && len(f.items) == 0 {
					ui.Row(c).Height(140).Center().Gap(12).Children(func() {
						ui.Spinner(c).FontSize(18)
						ui.Text(c, "Finding "+strings.ToLower(title)+"…").TextColor(inkSoft)
					})
					return
				}
				label := plural(f.total, "place", "places")
				if kind == artists {
					label = plural(f.total, "artist", "artists")
				} else if kind == museums {
					label = plural(f.total, "museum and cultural partner", "museums and cultural partners")
				}
				if q.Text != "" {
					label += " matching “" + q.Text + "”"
				}
				ui.Text(c, label).FontSize(12).TextColor(inkSoft)
				w, _ := c.Size()
				a.cultureGrid(c, f.items, w-80, true)
				switch {
				case f.err != nil:
					ui.Column(c).Padding(28, 0).AlignItems(ui.Center).Gap(10).Children(func() {
						msg := "The directory could not be reached. Check your connection and try again."
						if len(f.items) > 0 {
							msg = "The next page could not be loaded. Your current results are still here."
						}
						ui.Text(c, msg).TextColor(inkSoft)
						if ui.Button(c.Key("retry-directory"), "Try again").Clicked() {
							a.moreDestinations(f)
						}
					})
				case len(f.items) == 0:
					ui.Column(c).Padding(40, 0).AlignItems(ui.Center).Gap(12).Children(func() {
						ui.Text(c, "No "+strings.ToLower(title)+" found. Try another name or browse A–Z.").TextColor(inkSoft)
						if ui.Button(c, "Clear search").Clicked() {
							s.query = ""
						}
					})
				case f.loading:
					ui.Row(c).Height(64).Center().Children(func() { ui.Spinner(c).FontSize(18) })
				case !f.done:
					b := ui.Button(c.Key("more-destinations"), "Load more").AlignSelf(ui.Center)
					if b.Clicked() || b.InViewMargin(250) {
						a.moreDestinations(f)
					}
				}
				ui.Divider(c).Margin(8, 0, 0)
				ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
					ui.Text(c, "From Google Arts & Culture").FontSize(12).TextColor(inkFaint)
					ui.Spacer(c)
					link := cultureBase + "/partner"
					if category := kind.categoryID(); category != "" {
						link = cultureBase + "/category/" + category
					}
					externalButton(c, "Open directory in browser", link)
				})
			})
		})
	})
}

func letterButton(c *ui.Context, letter string, on bool, fn func()) {
	b := ui.ButtonBase(c).Size(27, 30).Radius(6).Label("Names beginning with " + letter).Selected(on).Cursor(ui.CursorPointer)
	if on {
		b.Background(ink).TextColor(paper)
	} else if b.Hovered() {
		b.Background(wash)
	} else {
		b.TextColor(inkSoft)
	}
	b.Children(func() { ui.Text(c, letter).FontSize(12).FontWeight(600) }).OnClick(fn)
}

func (a *app) cultureGrid(c *ui.Context, items []cultureItem, width float32, native bool) {
	cols := max(2, int((width+22)/(255+22)))
	w := (width - float32(cols-1)*22) / float32(cols)
	ui.Grid(c).Columns(cols).Gap(22).Children(func() {
		for _, item := range items {
			a.destinationCard(c.Key(item.Path), item, w, native)
		}
	})
}

func (a *app) openDestination(item cultureItem) {
	if cultureAssetID(item.Path) != "" {
		a.openCultureArtwork(item)
		return
	}
	if a.destinations[item.Path] == nil {
		a.destinations[item.Path] = &destinationState{seed: item}
	}
	// Preserve the index for Back; the router's fade controls the visual change.
	a.router.Push(item.route())
}

func (a *app) destinationCard(c *ui.Context, item cultureItem, w float32, native bool) {
	native = native || cultureAssetID(item.Path) != ""
	b := ui.ButtonBase(c).Column().AlignItems(ui.Stretch).Width(w).Gap(10).Cursor(ui.CursorPointer).Label(item.Name)
	if !native {
		b.Label("Open " + item.Name + " on Google Arts & Culture").Tooltip("Opens in your browser")
	}
	b.Children(func() {
		frame := ui.Box(c).Height(w / 1.4).Radius(8).Clip().Background(wash)
		frame.Children(func() {
			var pic *picture
			if frame.InViewMargin(400) {
				pic = a.img.get(item.Image, 900)
			}
			if pic != nil && pic.bmp != nil {
				ui.Image(c, pic.bmp).Fill().Fit(ui.Cover)
			} else {
				ui.Box(c).Fill().Center().Children(func() {
					ic := icMuseum
					if strings.HasPrefix(item.Path, "/entity/") {
						ic = icGlobe
						if strings.HasPrefix(item.route(), "/artists/") {
							ic = icArtist
						}
					}
					ui.Icon(c, ic).Size(40, 40).TextColor(inkFaint)
				})
			}
			if b.Hovered() {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.RGBA(20, 18, 15, 0.08))
			}
		})
		ui.Column(c).Height(68).Gap(3).Children(func() {
			ui.Text(c, item.Name).Font(serif).FontSize(20).LineHeight(1.15).MaxLines(2)
			if item.Subtitle != "" {
				ui.Text(c, item.Subtitle).FontSize(12).TextColor(inkSoft).SingleLine()
			}
		})
	})
	if b.Clicked() {
		if native {
			a.openDestination(item)
		} else {
			c.OpenURL(item.URL())
		}
	}
}

func (a *app) loadDestination(kind destinationKind, path string, s *destinationState) {
	s.loading, s.err = true, nil
	go func() {
		d, err := a.culture.Destination(context.Background(), kind, path)
		a.update(func() { s.detail, s.err, s.loading = d, err, false })
	}()
}

func (a *app) destinationPage(c *ui.Context, kind destinationKind, path string) {
	s := a.destinations[path]
	if s == nil {
		s = &destinationState{}
		a.destinations[path] = s
	}
	if s.detail == nil && !s.loading && s.err == nil {
		a.loadDestination(kind, path, s)
	}
	d := s.detail
	if d == nil {
		d = &destination{cultureItem: s.seed}
	}
	ui.Scroll(c.Key(path)).Fill().Children(func() {
		ui.Column(c).Padding(30, 40, 48).Gap(34).Children(func() {
			ui.Row(c).Gap(30).AlignItems(ui.Start).Children(func() {
				frame := ui.Box(c).WidthPercent(48).Height(360).Radius(12).Clip().Background(wash).Shrink(0)
				frame.Children(func() {
					p := a.img.get(d.Image, 1600)
					if p == nil || p.bmp == nil {
						p = a.img.get(s.seed.Image, 900)
					}
					if p != nil && p.bmp != nil {
						ui.Image(c, p.bmp).Fill().Fit(ui.Cover)
					}
				})
				ui.Column(c).Grow(1).Gap(14).Children(func() {
					label := "Museum & collection"
					if kind == artists {
						label = "Artist"
					} else if kind == places {
						label = "Place"
					}
					ui.Text(c, label).FontSize(12).TextColor(accent)
					ui.Text(c, d.Name).Font(serif).FontSize(34).LineHeight(1.1).Balance()
					if d.Subtitle != "" {
						ui.Text(c, d.Subtitle).FontSize(15).TextColor(inkSoft)
					}
					if s.loading {
						ui.Spinner(c).FontSize(18).AlignSelf(ui.Start)
					}
					if d.Description != "" {
						t := ui.Text(c, d.Description).FontSize(14).LineHeight(1.5).TextColor(inkSoft)
						if !s.expanded {
							t.MaxLines(6)
						}
						if len(d.Description) > 340 {
							label := "Read more"
							if s.expanded {
								label = "Show less"
							}
							if ui.Button(c.Key("description"), label).AlignSelf(ui.Start).Clicked() {
								s.expanded = !s.expanded
							}
						}
					}
					if d.SourceName != "" {
						externalButton(c, "Description: "+d.SourceName, d.SourceURL)
					}
					if d.Path != "" {
						externalButton(c, "Explore on Google Arts & Culture", d.URL())
					}
					if s.err != nil {
						ui.Text(c, "Details could not be loaded. Check your connection and try again.").TextColor(inkSoft)
						if ui.Button(c.Key("retry-destination"), "Try again").Clicked() {
							a.loadDestination(kind, path, s)
						}
					}
				})
			})
			if d.Address != "" || d.Website != "" {
				ui.Row(c).Padding(22, 24).Gap(24).Wrap().Background(wash).Radius(10).AlignItems(ui.Center).Children(func() {
					ui.Column(c).Grow(1).Gap(5).Children(func() {
						ui.Text(c, "Visit the museum").Font(serif).FontSize(24)
						if d.Address != "" {
							ui.Text(c, d.Address).TextColor(inkSoft)
						}
					})
					externalButton(c, "Museum website", d.Website)
					externalButton(c, "View on map", d.Maps)
				})
			}
			w, _ := c.Size()
			worksTitle, museumsTitle := "Collection highlights", "Museums & collections in "+d.Name
			if kind == artists {
				worksTitle, museumsTitle = "Selected works", "Museums & collections"
			}
			a.destinationSection(c.Key("works"), worksTitle, d.Works, w-80, false)
			a.destinationSection(c.Key("museums"), museumsTitle, d.Museums, w-80, true)
			a.destinationSection(c.Key("stories"), "Stories", d.Stories, w-80, false)
			if len(d.Visits) > 0 {
				ui.Column(c).Gap(14).Children(func() {
					ui.Text(c, "Virtual visits").Font(serif).FontSize(28)
					ui.Text(c, "Step inside with Street View in your browser.").TextColor(inkSoft)
					ui.Row(c).Wrap().Gap(10).Children(func() {
						for _, visit := range d.Visits {
							externalButton(c.Key(visit.Path), visit.Name, visit.URL())
						}
					})
				})
			}
			if s.detail != nil {
				ui.Row(c).Gap(12).Wrap().Children(func() {
					label := "Collection previews from Google Arts & Culture. Images belong to their respective owners."
					if d.Total > 0 {
						label = plural(d.Total, "item", "items") + " on Google Arts & Culture. Images belong to their respective owners."
					}
					ui.Text(c, label).FontSize(12).TextColor(inkFaint)
				})
			}
		})
	})
}

func (a *app) destinationSection(c *ui.Context, title string, items []cultureItem, width float32, native bool) {
	if len(items) == 0 {
		return
	}
	ui.Column(c).Gap(18).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Baseline).Children(func() {
			ui.Text(c, title).Font(serif).FontSize(28)
			if !native && cultureAssetID(items[0].Path) == "" {
				ui.Text(c, "Opens in your browser").FontSize(12).TextColor(inkFaint)
			}
		})
		a.cultureGrid(c, items, width, native)
	})
}

func externalButton(c *ui.Context, label, link string) {
	if link == "" {
		return
	}
	b := ui.ButtonBase(c).Padding(8, 0).Gap(6).AlignSelf(ui.Start).Cursor(ui.CursorPointer).Label(label + ", opens in browser").TextColor(accent)
	b.Children(func() {
		ui.Text(c, label).FontSize(12).FontWeight(500)
		ui.Icon(c, icExternal).Size(13, 13)
	})
	if b.Clicked() {
		c.OpenURL(link)
	}
}

func (a *app) discoveryLinks(c *ui.Context) {
	ui.Row(c).Padding(0, 40).Gap(20).Children(func() {
		for _, link := range []struct {
			kind       destinationKind
			title, sub string
			ic         *ui.SVG
		}{
			{museums, "Museums around the world", "Discover museums, galleries and their collections", icMuseum},
			{places, "A world of places", "Explore countries, cities and cultural landmarks", icGlobe},
		} {
			b := ui.ButtonBase(c.Key(link.kind)).Grow(1).Padding(24).Gap(18).Radius(10).Border(1, line).Cursor(ui.CursorPointer).Label(link.title)
			if b.Hovered() {
				b.Background(wash)
			}
			b.Children(func() {
				ui.Icon(c, link.ic).Size(30, 30).TextColor(accent).Shrink(0)
				ui.Column(c).Grow(1).Gap(5).Children(func() {
					ui.Text(c, link.title).Font(serif).FontSize(23)
					ui.Text(c, link.sub).FontSize(12).TextColor(inkSoft)
				})
				ui.Icon(c, icRight).Size(18, 18).TextColor(inkSoft)
			})
			if b.Clicked() {
				a.router.Push("/" + string(link.kind))
			}
		}
	})
}
