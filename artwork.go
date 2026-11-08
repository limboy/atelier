package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// viewer is where an artwork's picture is zoomed and panned to. zoom, tx
// and ty are where it is headed; sz, sx and sy what the frame shows now,
// gliding there after a click, a wheel notch or a key, and following a
// pinch or a drag at once.
type viewer struct {
	zoom   float32 // 1 fits the picture in the frame
	tx, ty float32 // DIPs the picture is moved, after zooming
	image  int     // which of the artwork's pictures shows
	hires  bool    // the full resolution picture was asked for

	sz, sx, sy float32
	vx, vy     float32 // a fling's DIPs per second, after a drag lets go
	moving     bool
	at         time.Time // the frame last stepped

	// The frame's last layout: its size, the picture's center at rest and
	// its size fitted in the frame, and how far it may zoom.
	w, h, cy, fw, fh, max float32

	pending *viewport // a region to show once the picture's shape is known
	tiles   *tileView // optional Google Arts & Culture detail pyramid
}

// viewport is a region of a picture, as shares of its width and height: its
// center and its size.
type viewport struct{ x, y, w, h float32 }

const maxZoom = 12

// layout records the frame w × h and the shape of the picture in it, as
// imageViewerPane lays it out: inset 36 DIPs, with 84 below for controls.
func (v *viewer) layout(w, h, aspect float32) {
	v.w, v.h, v.cy = w, h, h/2-24
	bw, bh := max(w-72, 1), max(h-120, 1)
	v.fw, v.fh = bw, bh
	if aspect > 0 {
		v.fw = min(bw, bh*aspect)
		v.fh = v.fw / aspect
	}
}

func (v *viewer) limit() float32 {
	if v.max <= 0 {
		return maxZoom
	}
	return v.max
}

// zoomAt zooms by factor about the point (x, y) of the frame.
func (v *viewer) zoomAt(factor, x, y float32) {
	z := min(max(v.zoom*factor, 1), v.limit())
	k := z / v.zoom
	qx, qy := x-v.w/2, y-v.cy
	v.tx = qx - k*(qx-v.tx)
	v.ty = qy - k*(qy-v.ty)
	v.zoom = z
	v.vx, v.vy = 0, 0
	v.clamp()
	if z > 1.25 {
		v.hires = true
	}
}

// fit shows the whole picture again.
func (v *viewer) fit() { v.zoom, v.tx, v.ty, v.vx, v.vy = 1, 0, 0, 0, 0 }

// pan moves the picture with the pointer or the fingers, at once.
func (v *viewer) pan(dx, dy float32) {
	v.tx += dx
	v.ty += dy
	v.sx += dx
	v.sy += dy
	v.clamp()
}

// snap shows where the picture is headed, without gliding there.
func (v *viewer) snap() { v.sz, v.sx, v.sy = v.zoom, v.tx, v.ty }

// centerOn moves the point (u, w) of the picture, in shares of its size, to
// the frame's center.
func (v *viewer) centerOn(u, w float32) {
	u, w = min(max(u, 0), 1), min(max(w, 0), 1)
	v.tx = -(u - 0.5) * v.fw * v.zoom
	v.ty = v.h/2 - v.cy - (w-0.5)*v.fh*v.zoom
	v.vx, v.vy = 0, 0
	v.clamp()
}

// show zooms to the region p, whole in the frame.
func (v *viewer) show(p viewport) {
	v.zoom = min(max(min(v.w/(v.fw*p.w), v.h/(v.fh*p.h)), 1), v.limit())
	v.hires = v.hires || v.zoom > 1.25
	v.centerOn(p.x, p.y)
}

// visible is the region of the picture the frame shows now, in shares of
// its size.
func (v *viewer) visible() (u0, w0, u1, w1 float32) {
	if v.fw <= 0 || v.fh <= 0 || v.sz <= 0 {
		return 0, 0, 1, 1
	}
	share := func(screen, center, shift, size float32) float32 {
		return min(max((screen-center-shift)/(size*v.sz)+0.5, 0), 1)
	}
	return share(0, v.w/2, v.sx, v.fw), share(0, v.cy, v.sy, v.fh),
		share(v.w, v.w/2, v.sx, v.fw), share(v.h, v.cy, v.sy, v.fh)
}

// clamp keeps the zoom within its limits and the picture over the frame.
func (v *viewer) clamp() {
	v.zoom = min(max(v.zoom, 1), v.limit())
	if v.zoom <= 1.001 {
		v.zoom, v.tx, v.ty = 1, 0, 0
		return
	}
	v.tx = clampPan(v.tx, v.zoom*v.fw, v.w, v.w/2)
	v.ty = clampPan(v.ty, v.zoom*v.fh, v.h, v.cy)
}

// clampPan keeps a picture s DIPs long, centered at c+t in a frame n DIPs
// long, covering the frame along that axis where it is longer, and where it
// is shorter, whole in the frame and as near its place at rest as it can be.
func clampPan(t, s, n, c float32) float32 {
	lo, hi := n-c-s/2, s/2-c
	if lo > hi {
		return min(max(0, hi), lo)
	}
	return min(max(t, lo), hi)
}

// step moves what the frame shows dt seconds on, and reports whether it
// still moves. still, for the desktop's Reduce Motion, shows the target at
// once and lets no fling go on.
func (v *viewer) step(dt float32, still bool) bool {
	if v.vx != 0 || v.vy != 0 {
		if still {
			v.vx, v.vy = 0, 0
		} else {
			wantX, wantY := v.tx+v.vx*dt, v.ty+v.vy*dt
			v.tx, v.ty = wantX, wantY
			v.clamp()
			// A fling stops at the picture's edges, and slows down.
			if v.tx != wantX {
				v.vx = 0
			}
			if v.ty != wantY {
				v.vy = 0
			}
			decay := float32(math.Exp(float64(-4 * dt)))
			v.vx, v.vy = v.vx*decay, v.vy*decay
			if math.Hypot(float64(v.vx), float64(v.vy)) < 20 {
				v.vx, v.vy = 0, 0
			}
			v.sx, v.sy = v.tx, v.ty
		}
	}
	if still || v.sz <= 0 {
		v.snap()
	} else {
		k := 1 - float32(math.Exp(float64(-18*dt)))
		v.sz += (v.zoom - v.sz) * k
		v.sx += (v.tx - v.sx) * k
		v.sy += (v.ty - v.sy) * k
		if abs32(v.zoom-v.sz) < 0.0005*v.zoom && abs32(v.tx-v.sx) < 0.2 && abs32(v.ty-v.sy) < 0.2 {
			v.snap()
		}
	}
	return v.vx != 0 || v.vy != 0 || v.sz != v.zoom || v.sx != v.tx || v.sy != v.ty
}

func abs32(f float32) float32 { return float32(math.Abs(float64(f))) }

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
	var pyramid *culturePyramid
	if v.tiles != nil {
		pyramid = v.tiles.pyramid
	}
	orig := pictures[v.image].full
	base := a.img.get(pictures[v.image].preview, 1800)
	pic := base
	var high *picture
	if v.hires && orig != "" {
		if pyramid != nil {
			high = a.img.m[picKey{orig, 5000}]
		} else {
			high = a.img.get(orig, 5000)
		}
		if high != nil && high.bmp != nil {
			pic = high
		}
	}

	pane := ui.Box(c.Key("viewer")).Grow(1).Clip().Background(night).Cursor(ui.CursorGrab)
	r := pane.Bounds()
	if r.W <= 0 || r.H <= 0 {
		c.AnimationFrame() // Bounds are available after the first layout.
	}
	var aspect float32
	if pic != nil && pic.bmp != nil {
		aspect = pic.Aspect()
	}
	if pyramid != nil {
		full := pyramid.levels[len(pyramid.levels)-1]
		aspect = float32(full.width) / float32(full.height)
	}
	v.layout(r.W, r.H, aspect)
	// Zoom in until a pixel of the best picture there is spans about two
	// DIPs, and as far as maxZoom while a larger one is on its way.
	v.max = maxZoom
	if pyramid != nil && v.fw > 0 {
		full := pyramid.levels[len(pyramid.levels)-1]
		v.max = max(2*float32(full.width)/v.fw, 4)
	} else if aspect > 0 && v.fw > 0 && !(high != nil && high.loading) && !(v.tiles != nil && v.tiles.loading) {
		v.max = min(max(2*float32(pic.w)/v.fw, 4), maxZoom)
	}
	if v.zoom > v.max {
		v.zoomAt(1, r.W/2, r.H/2)
	}
	if v.pending != nil && aspect > 0 {
		if r.W > 0 {
			v.show(*v.pending)
			if v.tiles == nil || !v.tiles.loading {
				v.pending = nil
			}
		} else {
			c.AnimationFrame() // for the frame's size
		}
	}
	if pane.Dragging() {
		pane.Cursor(ui.CursorGrabbing)
	}
	pane.HandleInput(func(ev ui.InputEvent) bool {
		switch {
		case ev.Kind == ui.InputZoom:
			v.zoomAt(ev.Scale, ev.X, ev.Y)
			v.snap()
		case ev.Kind == ui.InputScroll && ev.Precise && ev.Mods&ui.Ctrl == 0:
			if v.zoom > 1 {
				v.pan(-ev.DX, -ev.DY)
			} else {
				v.zoomAt(1-ev.DY*0.004, ev.X, ev.Y)
				v.snap()
			}
		case ev.Kind == ui.InputScroll:
			if ev.DY < 0 {
				v.zoomAt(1.25, ev.X, ev.Y)
			} else if ev.DY > 0 {
				v.zoomAt(0.8, ev.X, ev.Y)
			}
		default:
			return false
		}
		return true
	})
	if dx, dy, ok := pane.Dragged(); ok && v.zoom > 1 {
		v.pan(dx, dy)
	}
	if vx, vy, ok := pane.DragEnded(); ok && v.zoom > 1 {
		v.vx, v.vy = vx, vy
	}
	if pane.DoubleClicked() {
		if v.zoom > 1 {
			v.fit()
		} else if x, y, ok := pane.PointerPosition(); ok {
			v.zoomAt(2.5, x, y)
		}
	}
	// What the frame shows glides to where the picture is headed.
	dt := float32(1.0 / 60)
	if now := c.Now(); v.moving && !v.at.IsZero() {
		dt = min(max(float32(now.Sub(v.at).Seconds()), 0), 1.0/15)
		v.at = now
	} else {
		v.at = now
	}
	if v.moving = v.step(dt, c.Preferences().ReduceMotion); v.moving {
		c.AnimationFrame()
	}
	pane.Children(func() {
		if (pic == nil || pic.bmp == nil) && pyramid == nil {
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
			if pic != nil && pic.bmp != nil {
				ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
					RadialGradient(ui.RadialGradient{Stops: []ui.GradientStop{{Color: pic.tone.Alpha(0.35), At: 0}, {Color: ui.Transparent, At: 1}}})
			}
			img := ui.Box(c.Key("artwork-image")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Label(title).
				Draw(func(p *ui.Painter, frame ui.Rect) {
					if pic != nil && pic.bmp != nil {
						p.ImageRegion(pic.bmp, ui.Rect{}, v.pictureRect(frame), ui.ImageOptions{})
					}
					if a.tiles != nil {
						a.tiles.draw(p, frame, v)
					}
				})
			img.Opacity(img.AnimateTo("in", 1, ui.Animation{Duration: 250e6, NonMotion: true}))
			// Zoomed in, an overview shows the region in view.
			shown := float32(0)
			if v.sz > 1.01 {
				shown = 1
			}
			if op := pane.AnimateTo("navigator", shown, ui.Animation{Duration: 180e6, NonMotion: true}); op > 0.01 && pic != nil && pic.bmp != nil {
				navigator(c, pic, v, op)
			}
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
						*v = viewer{zoom: 1, image: i, w: v.w, h: v.h}
					}
				}
			})
		}

		// Zoom controls.
		ui.Row(c.Key("zoom")).Absolute().Right(20).Bottom(18).Gap(2).Padding(4).Radius(22).
			Background(ui.RGBA(20, 18, 16, 0.72)).Children(func() {
			viewerButton(c, icMinus, "Zoom out", func() { v.zoomAt(1/1.5, r.W/2, r.H/2) })
			// The zoom, which a click fits to the frame again.
			fit := ui.ButtonBase(c.Key("fit")).Width(52).Height(32).Radius(16).Cursor(ui.CursorPointer).
				Label("Fit to window").Tooltip("Fit to window").Disabled(v.zoom == 1).DimDisabled(false)
			if fit.Hovered() && v.zoom != 1 {
				fit.Background(ui.RGBA(255, 255, 255, 0.16))
			}
			fit.Children(func() {
				ui.Textf(c, "%d%%", int(v.zoom*100+0.5)).FontSize(12).FontFeatures("tnum").TextColor(ui.Hex("#d9d2c7"))
			}).OnClick(v.fit)
			viewerButton(c, icPlus, "Zoom in", func() { v.zoomAt(1.5, r.W/2, r.H/2) })
			if a.immersive {
				viewerButton(c, icShrink, "Exit full screen", func() { a.setImmersive(false) })
			} else {
				viewerButton(c, icExpand, "Full screen", func() { a.setImmersive(true) })
			}
		})
		tileLoading, tileFailed := false, false
		if a.tiles != nil && v.tiles != nil && v.hires {
			tileLoading, tileFailed = a.tiles.status(v.tiles)
		}
		if tileFailed {
			ui.Row(c.Key("tile-error")).Absolute().Right(20).Top(18).Gap(8).Padding(6, 12).Radius(14).
				Background(ui.RGBA(0, 0, 0, 0.65)).Children(func() {
				ui.Text(c, "Some detail could not load").FontSize(12).TextColor(ui.Hex("#d9d2c7"))
				if ui.Button(c.Key("retry-tiles"), "Retry detail").Clicked() {
					a.tiles.retry(v.tiles)
				}
			})
		} else if tileLoading || high != nil && high.loading && pic == base {
			ui.Row(c.Key("hires")).Absolute().Right(20).Top(18).Gap(8).Padding(6, 12).Radius(14).
				Background(ui.RGBA(0, 0, 0, 0.45)).TextColor(ui.Hex("#d9d2c7")).Children(func() {
				ui.Spinner(c).FontSize(12).TextColor(ui.Hex("#d9d2c7"))
				label := hiresLabel
				if tileLoading {
					label = "Loading detail"
				}
				ui.Text(c, label).FontSize(12)
			})
		}
	})
	c.OnWindowShortcut(ui.Cmd, ui.KeyEqual, func() { v.zoomAt(1.5, r.W/2, r.H/2) })
	c.OnWindowShortcut(ui.Cmd, ui.KeyMinus, func() { v.zoomAt(1/1.5, r.W/2, r.H/2) })
	c.OnWindowShortcut(ui.Cmd, ui.Key0, v.fit)
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

// navigator is the overview of a zoomed picture, at the frame's top left:
// the region in view is outlined, and a press or a drag moves the view.
func navigator(c *ui.Context, pic *picture, v *viewer, opacity float32) {
	const side = 156
	nw, nh := float32(side), float32(side)
	if aspect := pic.Aspect(); aspect >= 1 {
		nh = side / aspect
	} else {
		nw = side * aspect
	}
	n := ui.Box(c.Key("navigator")).Absolute().Left(20).Top(18).Size(nw, nh).Radius(4).Clip().
		Background(ui.Hex("#2a2622")).Border(1, ui.RGBA(255, 255, 255, 0.18)).Opacity(opacity).
		Cursor(ui.CursorPointer).Label("Overview").Tooltip("Overview: click or drag to move the view")
	n.Dragged()
	if n.Pressed() || n.Clicked() {
		x, y, _ := n.PointerPosition()
		v.centerOn(x/nw, y/nh)
	}
	u0, w0, u1, w1 := v.visible()
	x0, y0, x1, y1 := u0*nw, w0*nh, u1*nw, w1*nh
	n.Children(func() {
		ui.Image(c, pic.bmp).Fill().Fit(ui.Cover).PassThrough()
		dim := ui.RGBA(10, 9, 8, 0.55)
		ui.Box(c).Absolute().Left(0).Top(0).Size(nw, y0).Background(dim).PassThrough()
		ui.Box(c).Absolute().Left(0).Top(y1).Size(nw, nh-y1).Background(dim).PassThrough()
		ui.Box(c).Absolute().Left(0).Top(y0).Size(x0, y1-y0).Background(dim).PassThrough()
		ui.Box(c).Absolute().Left(x1).Top(y0).Size(nw-x1, y1-y0).Background(dim).PassThrough()
		ui.Box(c).Absolute().Left(x0).Top(y0).Size(x1-x0, y1-y0).Border(1.5, paper).PassThrough()
	})
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
