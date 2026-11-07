package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// viewer is where an artwork's picture is zoomed and panned to.
type viewer struct {
	zoom   float32 // 1 fits the picture in the frame
	tx, ty float32 // DIPs the picture is moved, after zooming
	image  int     // which of the artwork's pictures shows
	hires  bool    // the full resolution picture was asked for
}

const maxZoom = 12

// zoomAt zooms by factor about the point (x, y) of a frame w × h.
func (v *viewer) zoomAt(factor, x, y, w, h float32) {
	z := min(max(v.zoom*factor, 1), maxZoom)
	k := z / v.zoom
	qx, qy := x-w/2, y-h/2
	v.tx = qx - k*(qx-v.tx)
	v.ty = qy - k*(qy-v.ty)
	v.zoom = z
	v.clamp(w, h)
	if z > 1.25 {
		v.hires = true
	}
}

func (v *viewer) clamp(w, h float32) {
	if v.zoom <= 1.001 {
		v.zoom, v.tx, v.ty = 1, 0, 0
		return
	}
	mx, my := (v.zoom-1)*w/2+w*0.25, (v.zoom-1)*h/2+h*0.25
	v.tx = min(max(v.tx, -mx), mx)
	v.ty = min(max(v.ty, -my), my)
}

// webLarge is the Met's smaller rendition of one of its original pictures.
func webLarge(u string) string { return strings.Replace(u, "/original/", "/web-large/", 1) }

func (a *app) artworkPage(c *ui.Context, id int) {
	s := a.artwork(id)
	if s.loading {
		ui.Box(c).Fill().Center().Children(func() { ui.Spinner(c).FontSize(22) })
		return
	}
	if s.art == nil {
		ui.Column(c).Fill().Center().Gap(10).Children(func() {
			ui.Text(c, "This artwork could not be loaded.").FontSize(15)
			if ui.Button(c, "Try again").Clicked() {
				delete(a.arts, id)
			}
		})
		return
	}
	art := s.art
	v, ok := a.viewers[id]
	if !ok {
		v = &viewer{zoom: 1}
		a.viewers[id] = v
	}
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		a.viewerPane(c, art, v)
		ui.Scroll(c.Key("details")).Hidden(a.immersive).Width(420).Shrink(0).Background(paper).BorderWidth(0, 0, 0, 1).BorderColor(line).Children(func() {
			a.details(c, art)
		})
	})
}

func (a *app) viewerPane(c *ui.Context, art *Artwork, v *viewer) {
	urls := append([]string{art.Image}, art.AdditionalImages...)
	if art.Image == "" {
		urls = []string{art.ImageSmall}
	}
	pictures := make([]viewerImage, len(urls))
	for i, u := range urls {
		pictures[i] = viewerImage{preview: webLarge(u), full: u}
		if i == 0 && art.ImageSmall != "" {
			pictures[i].preview = art.ImageSmall
		}
	}
	a.imageViewerPane(c, cleanTitle(art.Title), pictures, v, "Loading full resolution")
}

type viewerImage struct{ preview, full string }

// imageViewerPane is shared by Met and Google Arts & Culture artworks.
func (a *app) imageViewerPane(c *ui.Context, title string, pictures []viewerImage, v *viewer, hiresLabel string) {
	if len(pictures) == 0 {
		pictures = []viewerImage{{}}
	}
	if v.zoom == 0 {
		v.zoom = 1
	}
	v.image = min(v.image, len(pictures)-1)
	orig := pictures[v.image].full
	base := a.img.get(pictures[v.image].preview, 1800)
	pic := base
	var high *picture
	if v.hires && orig != "" {
		high = a.img.get(orig, 5000)
		if high != nil && high.bmp != nil {
			pic = high
		}
	}

	pane := ui.Box(c.Key("viewer")).Grow(1).Clip().Background(night).Cursor(ui.CursorGrab)
	r := pane.Bounds()
	if pane.Dragging() {
		pane.Cursor(ui.CursorGrabbing)
	}
	pane.HandleInput(func(ev ui.InputEvent) bool {
		switch {
		case ev.Kind == ui.InputZoom:
			v.zoomAt(ev.Scale, ev.X, ev.Y, r.W, r.H)
		case ev.Kind == ui.InputScroll && ev.Precise && ev.Mods&ui.Ctrl == 0:
			if v.zoom > 1 {
				v.tx -= ev.DX
				v.ty -= ev.DY
				v.clamp(r.W, r.H)
			} else {
				v.zoomAt(1-ev.DY*0.004, ev.X, ev.Y, r.W, r.H)
			}
		case ev.Kind == ui.InputScroll:
			if ev.DY < 0 {
				v.zoomAt(1.25, ev.X, ev.Y, r.W, r.H)
			} else if ev.DY > 0 {
				v.zoomAt(0.8, ev.X, ev.Y, r.W, r.H)
			}
		default:
			return false
		}
		return true
	})
	if dx, dy, ok := pane.Dragged(); ok && v.zoom > 1 {
		v.tx += dx
		v.ty += dy
		v.clamp(r.W, r.H)
	}
	if pane.DoubleClicked() {
		if v.zoom > 1 {
			v.zoom, v.tx, v.ty = 1, 0, 0
		} else if x, y, ok := pane.PointerPosition(); ok {
			v.zoomAt(2.5, x, y, r.W, r.H)
		}
	}
	pane.Children(func() {
		if pic == nil || pic.bmp == nil {
			if pic == nil {
				ui.Text(c, "No picture is available for this artwork.").TextColor(inkFaint).Attach(ui.AnchorCenter, ui.AnchorCenter)
			} else if pic.failed {
				ui.Column(c).Gap(10).AlignItems(ui.Center).Attach(ui.AnchorCenter, ui.AnchorCenter).Children(func() {
					ui.Text(c, "The picture could not be loaded.").TextColor(inkFaint)
					if ui.Button(c.Key("retry-image"), "Retry image").Clicked() {
						delete(a.img.m, picKey{pictures[v.image].preview, 1800})
					}
				})
			} else {
				ui.Spinner(c).FontSize(22).TextColor(inkFaint).Attach(ui.AnchorCenter, ui.AnchorCenter)
			}
		} else {
			// The picture's own tone glows faintly around it.
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
				RadialGradient(ui.RadialGradient{Stops: []ui.GradientStop{{Color: pic.tone.Alpha(0.35), At: 0}, {Color: ui.Transparent, At: 1}}})
			img := ui.Image(c.Key(pic), pic.bmp).Absolute().Top(36).Left(36).Right(36).Bottom(84).Fit(ui.Contain).
				Scale(v.zoom, v.zoom).Translate(v.tx, v.ty).TransformOrigin(0.5, 0.5).PassThrough().
				Label(title)
			img.Opacity(img.AnimateTo("in", 1, ui.Animation{Duration: 250e6, NonMotion: true}))
		}

		// Thumbnails of the artwork's other pictures.
		if len(pictures) > 1 {
			ui.ScrollHorizontal(c.Key("thumbs")).Absolute().Left(20).Bottom(18).MaxWidthPercent(60).Gap(8).Scrollbars(false).Children(func() {
				for i, image := range pictures {
					t := a.img.get(image.preview, 240)
					b := ui.ButtonBase(c.Key(i)).Size(48, 48).Radius(6).Clip().Shrink(0).Background(ui.Hex("#2a2622")).
						Cursor(ui.CursorPointer).Label("Picture " + strconv.Itoa(i+1))
					if i == v.image {
						b.Border(2, paper)
					} else if !b.Hovered() {
						b.Opacity(0.6)
					}
					b.Children(func() {
						if t != nil && t.bmp != nil {
							ui.Image(c, t.bmp).Fill().Fit(ui.Cover)
						}
					})
					if b.Clicked() && i != v.image {
						*v = viewer{zoom: 1, image: i}
					}
				}
			})
		}

		// Zoom controls.
		ui.Row(c.Key("zoom")).Absolute().Right(20).Bottom(18).Gap(2).Padding(4).Radius(22).
			Background(ui.RGBA(20, 18, 16, 0.72)).Children(func() {
			viewerButton(c, icMinus, "Zoom out", func() { v.zoomAt(1/1.5, r.W/2, r.H/2, r.W, r.H) })
			// The zoom, which a click fits to the frame again.
			fit := ui.ButtonBase(c.Key("fit")).Width(52).Height(32).Radius(16).Cursor(ui.CursorPointer).
				Label("Fit to window").Tooltip("Fit to window").Disabled(v.zoom == 1).DimDisabled(false)
			if fit.Hovered() && v.zoom != 1 {
				fit.Background(ui.RGBA(255, 255, 255, 0.16))
			}
			fit.Children(func() {
				ui.Textf(c, "%d%%", int(v.zoom*100+0.5)).FontSize(12).FontFeatures("tnum").TextColor(ui.Hex("#d9d2c7"))
			}).OnClick(func() { v.zoom, v.tx, v.ty = 1, 0, 0 })
			viewerButton(c, icPlus, "Zoom in", func() { v.zoomAt(1.5, r.W/2, r.H/2, r.W, r.H) })
			if a.immersive {
				viewerButton(c, icShrink, "Exit full screen", func() { a.setImmersive(false) })
			} else {
				viewerButton(c, icExpand, "Full screen", func() { a.setImmersive(true) })
			}
		})
		if high != nil && high.loading && pic == base {
			ui.Row(c.Key("hires")).Absolute().Right(20).Top(18).Gap(8).Padding(6, 12).Radius(14).
				Background(ui.RGBA(0, 0, 0, 0.45)).TextColor(ui.Hex("#d9d2c7")).Children(func() {
				ui.Spinner(c).FontSize(12).TextColor(ui.Hex("#d9d2c7"))
				ui.Text(c, hiresLabel).FontSize(12)
			})
		}
	})
	c.OnWindowShortcut(ui.Cmd, ui.KeyEqual, func() { v.zoomAt(1.5, r.W/2, r.H/2, r.W, r.H) })
	c.OnWindowShortcut(ui.Cmd, ui.KeyMinus, func() { v.zoomAt(1/1.5, r.W/2, r.H/2, r.W, r.H) })
	c.OnWindowShortcut(ui.Cmd, ui.Key0, func() { v.zoom, v.tx, v.ty = 1, 0, 0 })
	c.OnWindowShortcut(0, ui.KeyF, func() { a.setImmersive(!a.immersive) })
	// Esc first leaves full screen, then the artwork itself, but not while
	// typing: a search field clears itself with Esc, and an empty one keeps
	// the page.
	if a.immersive || !c.FocusTakesText() {
		c.OnWindowShortcut(0, ui.KeyEscape, func() {
			switch {
			case a.immersive:
				a.setImmersive(false)
			case a.router.CanGoBack():
				a.router.Back()
			default:
				a.router.Push("/home")
			}
		})
	}
}

func viewerButton(c *ui.Context, ic *ui.SVG, label string, fn func()) {
	b := ui.ButtonBase(c.Key(label)).Size(32, 32).Radius(16).TextColor(paper).Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	if b.Hovered() {
		b.Background(ui.RGBA(255, 255, 255, 0.16))
	}
	b.Children(func() { ui.Icon(c, ic).Size(16, 16) }).OnClick(fn)
}

func (a *app) favoriteButton(c *ui.Context, id int) {
	on := a.favs.has(id)
	label := "Add to favorites"
	ic := icHeart
	if on {
		label, ic = "In your favorites", icHeartOn
	}
	b := ui.Button(c.Key("fav"), "").Padding(0, 14).Height(34).Radius(17)
	b.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			pop := b.AnimateTo("pop", map[bool]float32{true: 1, false: 0}[on], ui.Bouncy)
			ui.Icon(c, ic).Size(16, 16).TextColor(map[bool]ui.Color{true: accent, false: ink}[on]).
				Scale(1+0.15*pop*(1-pop)*4, 1+0.15*pop*(1-pop)*4)
			ui.Text(c, label)
		})
	})
	if b.Clicked() {
		a.favs.toggle(id)
	}
}

func (a *app) details(c *ui.Context, art *Artwork) {
	ui.Column(c).Padding(32, 30, 40).Gap(22).Children(func() {
		ui.Column(c).Gap(10).Children(func() {
			if art.Department != "" {
				ui.Text(c, strings.ToUpper(art.Department)).FontSize(11).FontWeight(600).LetterSpacing(1.4).TextColor(accent)
			}
			ui.Text(c, cleanTitle(art.Title)).Font(serif).FontSize(30).LineHeight(1.15).Selectable().Pretty()
			if art.Artist != "" {
				l := ui.ButtonBase(c.Key("artist")).AlignSelf(ui.Start).Cursor(ui.CursorPointer)
				col := ink
				if l.Hovered() {
					col = accent
				}
				l.Children(func() { ui.Text(c, art.Artist).FontSize(16).FontWeight(500).TextColor(col) })
				if l.Clicked() {
					a.router.Push(collectionPath(art.Artist, "Artist", Query{Q: art.Artist, Artist: true}))
				}
				if art.ArtistBio != "" {
					ui.Text(c, art.ArtistBio).TextColor(inkSoft)
				}
			} else if art.Culture != "" {
				ui.Text(c, art.Culture).FontSize(16).FontWeight(500)
			}
			if art.Date != "" {
				ui.Text(c, art.Date).TextColor(inkSoft)
			}
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			a.favoriteButton(c, art.ID)
			if art.URL != "" {
				b := ui.Button(c.Key("met"), "").Padding(0, 14).Height(34).Radius(17)
				b.Children(func() {
					ui.Row(c).Gap(6).Children(func() {
						ui.Icon(c, icExternal).Size(15, 15)
						ui.Text(c, "View at The Met")
					})
				})
				if b.Clicked() {
					c.OpenURL(art.URL)
				}
			}
		})
		ui.Divider(c)
		ui.Text(c, "Details").Font(serif).FontSize(20)
		ui.Grid(c).ColumnTracks(ui.Fixed(110), ui.Fr(1)).GapX(14).GapY(10).Selectable().Children(func() {
			row := func(k, v string) {
				if strings.TrimSpace(v) == "" {
					return
				}
				ui.Text(c, k).FontSize(12).TextColor(inkFaint)
				ui.Text(c, v).FontSize(13)
			}
			row("Title", cleanTitle(art.Title))
			row("Creator", art.Artist)
			row("Date", art.Date)
			row("Culture", art.Culture)
			row("Period", strings.Trim(art.Period+" · "+art.Dynasty, " ·"))
			row("Type", art.ObjectName)
			row("Medium", art.Medium)
			row("Dimensions", art.Dimensions)
			row("Place", strings.Trim(strings.Join([]string{art.City, art.Region, art.Country}, ", "), ", "))
			row("Classification", art.Classification)
			row("Credit", art.CreditLine)
			row("Accession", art.Accession)
		})
		if len(art.Tags) > 0 {
			ui.Column(c).Gap(10).Children(func() {
				ui.Text(c, "Themes").Font(serif).FontSize(20)
				ui.Row(c).Gap(6).Wrap().Children(func() {
					for _, t := range art.Tags {
						chip(c.Key(t.Term), t.Term, false, func() {
							a.router.Push(collectionPath(t.Term, "Theme", Query{Q: t.Term}))
						})
					}
				})
			})
		}
		if art.Artist != "" {
			a.related(c, "More by "+art.Artist, Query{Q: art.Artist, Artist: true}, art.ID)
		}
		if art.Department != "" {
			for id, name := range departments {
				if name == art.Department {
					a.related(c, "More from "+name, Query{Department: id, Highlight: true}, art.ID)
				}
			}
		}
	})
}

// related is a small grid of other works.
func (a *app) related(c *ui.Context, title string, q Query, except int) {
	f := a.feed(q, 12)
	var items []*Artwork
	for _, it := range f.items {
		if it.ID != except && len(items) < 6 {
			items = append(items, it)
		}
	}
	if len(items) == 0 {
		return
	}
	ui.Column(c.Key(title)).Gap(12).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, title).Font(serif).FontSize(20).Grow(1).MaxLines(2)
		})
		ui.Grid(c).Columns(3).Gap(10).Children(func() {
			for _, it := range items {
				p := a.img.get(it.Thumb(), 360)
				b := ui.ButtonBase(c.Key(it.ID)).AspectRatio(1).Radius(6).Clip().Background(wash).Cursor(ui.CursorPointer).
					Label(cleanTitle(it.Title)).Tooltip(cleanTitle(it.Title))
				b.Children(func() {
					if p != nil && p.bmp != nil {
						s := b.AnimateTo("h", map[bool]float32{true: 1.06, false: 1}[b.Hovered()], ui.Smooth)
						ui.Image(c, p.bmp).Fill().Fit(ui.Cover).Scale(s, s)
					}
				})
				if b.Clicked() {
					a.router.Push("/artwork/" + strconv.Itoa(it.ID))
				}
			}
		})
	})
}

var departments = map[int]string{
	1: "The American Wing", 3: "Ancient West Asian Art", 4: "Arms and Armor",
	5: "Arts of Africa, Oceania, and the Americas", 6: "Asian Art", 7: "The Cloisters",
	8: "The Costume Institute", 9: "Drawings and Prints", 10: "Egyptian Art",
	11: "European Paintings", 12: "European Sculpture and Decorative Arts",
	13: "Greek and Roman Art", 14: "Islamic Art", 15: "The Robert Lehman Collection",
	16: "The Libraries", 17: "Medieval Art", 18: "Musical Instruments",
	19: "Photographs", 21: "Modern Art",
}
