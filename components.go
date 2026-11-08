package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// topBarHeight is the height of the top bar, which the window's traffic
// lights are centered in.
const topBarHeight = 56

// topBar is the bar along the top of the window, under the window's
// buttons: history, the app's name, its sections and the search field.
func (a *app) topBar(c *ui.Context) {
	bar := c.TitleBar()
	path := a.router.Path()
	w, _ := c.Size()
	ui.Row(c).Height(max(bar.Height, topBarHeight)).Shrink(0).Padding(0, bar.Right+16, 0, bar.Left+14).
		Gap(6).Background(paper).BorderWidth(0, 0, 1, 0).BorderColor(line).
		DragWindow().Children(func() {
		ui.BackButton(c, a.router)
		ui.ForwardButton(c, a.router)
		ui.ButtonBase(c.Key("logo")).Gap(8).Padding(4, 10, 4, 6).Radius(8).Children(func() {
			ui.Image(c, logoMark).Size(24, 24)
			ui.Text(c, "Atelier").Font(serif).FontSize(19).FontWeight(600).LetterSpacing(0.2)
		}).Label("Atelier, home").OnClick(func() { a.router.Push("/home") })
		ui.Box(c).Width(12)
		a.navItem(c, "/home", icHome, "Home", path == "/home")
		a.navItem(c, "/artists", icArtist, "Artists", strings.HasPrefix(path, "/artists") || path == "/explore")
		a.navItem(c, "/museums", icMuseum, "Museums", strings.HasPrefix(path, "/museums"))
		a.navItem(c, "/places", icGlobe, "Places", strings.HasPrefix(path, "/places"))
		a.navItem(c, "/time", icClock, "Time machine", path == "/time")
		a.navItem(c, "/favorites", icHeart, "Favorites", path == "/favorites")
		ui.Spacer(c)
		f := ui.SearchField(c.Key("search"), &a.query).Placeholder("Search artworks or paste a link…").
			Label("Search artworks or open a link").Tooltip("Search The Met, or paste a Google Arts & Culture artwork link").Width(max(160, min(220, w-1080))).Radius(18)
		if f.Submitted() && strings.TrimSpace(a.query) != "" {
			q := strings.TrimSpace(a.query)
			if cultureAssetID(q) != "" {
				a.openCultureArtwork(cultureItem{Path: q})
			} else {
				a.router.Push("/search?q=" + urlQuery(q))
			}
		}
		if path != "/artists" && path != "/explore" && path != "/museums" && path != "/places" && c.Shortcut(ui.Cmd, ui.KeyF) {
			f.Focus()
		}
		gear := ui.ButtonBase(c.Key("settings")).Size(34, 34).Radius(17).Cursor(ui.CursorPointer).
			Label("Settings").Tooltip("Settings").Selected(path == "/settings")
		switch {
		case path == "/settings":
			gear.Background(ink).TextColor(paper)
		case gear.Hovered():
			gear.Background(wash).TextColor(ink)
		default:
			gear.TextColor(inkSoft)
		}
		gear.Children(func() { ui.Icon(c, icGear).Size(17, 17) })
		if gear.Clicked() && path != "/settings" {
			a.router.Push("/settings")
		}
	})
}

func (a *app) navItem(c *ui.Context, path string, ic *ui.SVG, label string, on bool) {
	w, _ := c.Size()
	b := ui.ButtonBase(c.Key(path)).Gap(6).Padding(7, 10).Radius(18).Cursor(ui.CursorPointer).Label(label).Tooltip(label)
	color := inkSoft
	switch {
	case on:
		b.Background(ink)
		color = paper
	case b.Hovered():
		b.Background(wash)
		color = ink
	}
	b.TextColor(color).Selected(on).Transition(ui.ElementTransition{Colors: true})
	b.Children(func() {
		ui.Icon(c, ic).Size(16, 16)
		if w >= 1220 {
			ui.Text(c, label).FontWeight(500)
		}
	})
	if b.Clicked() && !on {
		a.router.Push(path)
	}
}

func urlQuery(s string) string {
	r := strings.NewReplacer("%", "%25", "&", "%26", "+", "%2B", "#", "%23", "?", "%3F", " ", "+")
	return r.Replace(s)
}

// sectionHeader is a heading over a row of artworks, with a link to see
// them all.
func (a *app) sectionHeader(c *ui.Context, kicker, title, more string) {
	ui.Row(c).AlignItems(ui.End).Gap(16).Padding(0, 40).Children(func() {
		ui.Column(c).Gap(4).Grow(1).Children(func() {
			if kicker != "" {
				ui.Text(c, strings.ToUpper(kicker)).FontSize(11).FontWeight(600).LetterSpacing(1.4).TextColor(accent)
			}
			ui.Text(c, title).Font(serif).FontSize(26).FontWeight(500)
		})
		if more != "" {
			l := ui.ButtonBase(c.Key("more")).Gap(2).Padding(6, 4).Cursor(ui.CursorPointer)
			col := inkSoft
			if l.Hovered() {
				col = accent
			}
			l.TextColor(col).Children(func() {
				ui.Text(c, "View all").FontWeight(500)
				ui.Icon(c, icRight).Size(14, 14)
			}).OnClick(func() { a.router.Push(more) })
		}
	})
}

// strip is a row of artworks that scrolls sideways, each as high as h.
func (a *app) strip(c *ui.Context, f *feed, h float32, limit int) {
	ui.ScrollHorizontal(c).Scrollbars(false).Padding(0, 40).Gap(18).Shrink(0).AlignItems(ui.Start).Children(func() {
		items := f.items
		if limit > 0 && len(items) > limit {
			items = items[:limit]
		}
		for _, art := range items {
			a.card(c.Key(art.ID), art, h, 0)
		}
		if len(items) == 0 && f.err != nil {
			ui.Column(c).Height(h).Justify(ui.Center).Gap(8).Children(func() {
				ui.Text(c, "This row could not be loaded.").TextColor(inkSoft)
				if ui.Button(c, "Try again").Clicked() {
					f.err, f.done = nil, false
					a.more(f)
				}
			})
		} else if len(items) == 0 {
			for i := range 6 {
				skeleton(c.Key(i), h*(0.7+0.12*float32(i%3)), h)
			}
		}
	})
}

func skeleton(c *ui.Context, w, h float32) {
	ui.Column(c).Gap(8).Shrink(0).Children(func() {
		ui.Box(c).Size(w, h).Radius(4).Background(wash).Opacity(0.6 + 0.4*pulse(c))
		ui.Box(c).Size(w*0.7, 10).Radius(3).Background(wash)
		ui.Box(c).Size(w*0.4, 10).Radius(3).Background(wash)
	})
}

// pulse is a value going between 0 and 1, for things loading.
func pulse(c *ui.Context) float32 {
	return c.Root().Loop("pulse", 1400e6, ui.EaseInOut)
}

// card is an artwork's picture with its title and artist under it. Its
// picture is h high, as wide as its proportions say, or w wide when w is
// set, as high as they say.
func (a *app) card(c *ui.Context, art *Artwork, h, w float32) ui.Element {
	pic := a.img.get(art.Thumb(), 720)
	aspect := pic.Aspect()
	if aspect == 0 {
		aspect = 0.8
	}
	iw, ih := h*aspect, h
	if w > 0 {
		iw, ih = w, w/aspect
	}
	iw = min(iw, max(h*1.8, w))
	b := ui.ButtonBase(c).Column().AlignItems(ui.Start).Gap(10).Shrink(0).Width(iw).Cursor(ui.CursorPointer).
		Label(cleanTitle(art.Title) + ", " + art.Byline())
	hover := b.Hovered()
	b.Children(func() {
		frame := ui.Box(c).Size(iw, ih).Radius(4).Clip().Background(wash)
		if pic != nil && pic.bmp != nil {
			frame.Background(pic.tone)
		}
		lift := frame.AnimateTo("lift", map[bool]float32{true: 1, false: 0}[hover], ui.Smooth)
		frame.Shadow(0, 2+8*lift, 6+18*lift, 0, ui.RGBA(30, 20, 10, 0.10+0.12*lift)).Translate(0, -3*lift)
		frame.Children(func() {
			if pic != nil && pic.bmp != nil {
				img := ui.Image(c, pic.bmp).Fill().Fit(ui.Cover)
				img.Opacity(img.AnimateTo("in", 1, ui.Animation{Duration: 300e6, NonMotion: true}))
			}
		})
		ui.Column(c).Gap(2).Width(iw).Children(func() {
			ui.Text(c, cleanTitle(art.Title)).FontWeight(500).MaxLines(2).TextColor(ink)
			ui.Text(c, art.Byline()).FontSize(12).SingleLine().TextColor(inkSoft)
		})
	})
	if b.Clicked() {
		a.router.Push("/artwork/" + strconv.Itoa(art.ID))
	}
	return b
}

// masonryState places a feed's items in columns, each the first time it
// shows, so that items stay where they are as pictures arrive.
type masonryState struct {
	cols    int
	heights []float32
	col     map[int]int
	order   [][]*Artwork
	placed  int
}

// masonry lays artworks out in columns of about colW DIPs, in width w.
func (a *app) masonry(c *ui.Context, key string, items []*Artwork, w float32) {
	const gap = 22
	cols := max(2, int((w+gap)/(240+gap)))
	colW := (w - gap*float32(cols-1)) / float32(cols)
	m := a.masonryLayouts(key)
	if m.cols != cols || m.placed > len(items) {
		*m = masonryState{cols: cols, heights: make([]float32, cols), col: map[int]int{}, order: make([][]*Artwork, cols)}
	}
	for _, art := range items[m.placed:] {
		best := 0
		for i, h := range m.heights {
			if h < m.heights[best]-1 {
				best = i
			}
		}
		aspect := a.img.get(art.Thumb(), 720).Aspect()
		if aspect == 0 {
			aspect = 0.8
		}
		m.heights[best] += colW/aspect + 60
		m.order[best] = append(m.order[best], art)
	}
	m.placed = len(items)
	ui.Row(c).AlignItems(ui.Start).Gap(gap).Children(func() {
		for i, col := range m.order {
			ui.Column(c.Key(i)).Width(colW).Gap(28).Children(func() {
				for _, art := range col {
					a.card(c.Key(art.ID), art, 0, colW)
				}
			})
		}
	})
}

func (a *app) masonryLayouts(key string) *masonryState {
	m, ok := a.layouts[key]
	if !ok {
		m = &masonryState{}
		a.layouts[key] = m
	}
	return m
}

// feedFooter shows that more is loading, or loads it once it scrolls near.
func (a *app) feedFooter(c *ui.Context, f *feed) {
	ui.Row(c).Height(80).Center().Children(func() {
		switch {
		case f.err != nil && len(f.items) == 0:
			ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "The collection could not be reached.").TextColor(inkSoft)
				if ui.Button(c, "Try again").Clicked() {
					f.err, f.done = nil, false
					a.more(f)
				}
			})
		case f.loading:
			ui.Spinner(c).FontSize(18)
		case f.done && len(f.items) == 0:
			ui.Text(c, "Nothing here yet.").TextColor(inkSoft)
		case f.done:
			ui.Text(c, "· · ·").TextColor(inkFaint)
		}
	}).Key("footer")
	if !f.loading && !f.done && f.err == nil {
		sentinel := ui.Box(c.Key("sentinel")).Height(1)
		if sentinel.InViewMargin(900) {
			a.more(f)
		}
	}
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%s %s", groupDigits(n), many)
}

func groupDigits(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
