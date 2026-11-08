package main

import (
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// randomPeriod is how long each artwork stays.
const randomPeriod = 10 * time.Second

// randomState is the slideshow's: the artworks drawn from the home
// page's stories in random order, never twice, the one on show, the one
// loading to follow, and the one fading out. Read and written on the main
// thread.
type randomState struct {
	order   []int // story indexes, shuffled; sources is how many are drawn from
	sources int
	seen    map[int]bool
	skipped map[int]bool // artworks whose picture would not load

	cur, next, prev *Artwork
	since           time.Time
	paused          bool
	options         bool      // the options show
	lastMove        time.Time // the pointer last moved, for hiding the controls
}

// newRandomRun begins a run from what the options chose.
func (a *app) newRandomRun() {
	a.show = &randomState{
		order:    rand.Perm(len(stories)),
		sources:  3,
		seen:     map[int]bool{},
		skipped:  map[int]bool{},
		lastMove: time.Now(),
	}
}

// randomPicture is the picture shown for an artwork: large, but not the
// original.
func randomPicture(art *Artwork) string {
	if art.Image != "" {
		return webLarge(art.Image)
	}
	return art.ImageSmall
}

// pick chooses an artwork not shown yet, loading more of the stories when
// few are left, or nil while there is none to show.
func (s *randomState) pick(a *app) *Artwork {
	var left []*Artwork
	for _, i := range s.order[:s.sources] {
		for _, art := range a.feed(stories[i].q, 12).items {
			if !s.seen[art.ID] && !s.skipped[art.ID] && randomPicture(art) != "" {
				left = append(left, art)
			}
		}
	}
	if len(left) < 4 {
		if s.sources < len(s.order) {
			s.sources++
		}
		for _, i := range s.order[:s.sources] {
			a.more(a.feed(stories[i].q, 12))
		}
	}
	if len(left) == 0 {
		// All shown: start over once nothing more is on its way.
		loading := false
		for _, i := range s.order[:s.sources] {
			loading = loading || a.feed(stories[i].q, 12).loading
		}
		if !loading && len(s.seen) > 0 {
			clear(s.seen)
		}
		return nil
	}
	art := left[rand.IntN(len(left))]
	s.seen[art.ID] = true
	return art
}

// advance shows the artwork loaded to follow, if it has loaded.
func (s *randomState) advance(a *app, now time.Time) {
	if s.next == nil {
		return
	}
	if pic := a.img.get(randomPicture(s.next), 2400); pic != nil && pic.bmp != nil {
		s.prev, s.cur, s.next = s.cur, s.next, nil
		s.since = now
	}
}

func (a *app) randomPage(c *ui.Context) {
	if a.show == nil {
		a.newRandomRun() // the page shows at once what it opens on
	}
	s := a.show
	now := c.Now()

	// Keep the next artwork loading, and skip any whose picture fails.
	if s.next == nil {
		s.next = s.pick(a)
	}
	if s.next != nil {
		if pic := a.img.get(randomPicture(s.next), 2400); pic != nil && pic.failed {
			s.skipped[s.next.ID] = true
			s.next = nil
		}
	}
	switch {
	case s.cur == nil:
		s.advance(a, now)
	case !s.paused && now.Sub(s.since) >= randomPeriod:
		s.advance(a, now)
	}
	if s.cur != nil && !s.paused {
		c.After(max(randomPeriod-now.Sub(s.since), time.Second/10))
	}
	if s.cur == nil || s.next == nil {
		c.After(time.Second / 2) // waiting on the network
	}

	controls := now.Sub(s.lastMove) < 3*time.Second || s.paused
	if now.Sub(s.lastMove) < 3*time.Second {
		c.After(3*time.Second - now.Sub(s.lastMove) + 50*time.Millisecond)
	}

	stage := ui.Box(c.Key("stage")).Fill().Background(night).Clip()
	if !controls {
		stage.Cursor(ui.CursorNone)
	}
	stage.HandleInput(func(ev ui.InputEvent) bool {
		if ev.Kind == ui.InputPointerMove {
			s.lastMove = time.Now()
		}
		return false
	})
	stage.Children(func() {
		for _, art := range []*Artwork{s.prev, s.cur} {
			if art == nil {
				continue
			}
			age := randomPeriod // the one fading out stays as it drifted to
			if art == s.cur {
				age = now.Sub(s.since)
			}
			a.slide(c, art, art == s.cur, age)
		}
		if s.cur == nil {
			ui.Spinner(c).FontSize(22).TextColor(inkFaint).Attach(ui.AnchorCenter, ui.AnchorCenter)
			return
		}
		art := s.cur
		ui.Box(c.Key("shade")).Absolute().Left(0).Right(0).Bottom(0).Height(220).PassThrough().
			Gradient(ui.RGBA(14, 12, 10, 0), ui.RGBA(14, 12, 10, 0.85), 90)
		cap := ui.Column(c.Key("caption")).Absolute().Left(56).Bottom(52).MaxWidthPercent(60).Gap(6).PassThrough()
		fade := cap.AnimateTo("in", 1, ui.Animation{Duration: 900e6, NonMotion: true})
		cap.Opacity(fade).Children(func() {
			ui.Text(c, cleanTitle(art.Title)).Font(serif).FontSize(30).LineHeight(1.1).TextColor(paper).MaxLines(2)
			ui.Text(c, art.Byline()+dateSuffix(art.Date)).FontSize(15).TextColor(ui.Hex("#d9d2c7"))
		})

		bar := ui.Row(c.Key("controls")).Absolute().Right(40).Bottom(44).Gap(2).Padding(4).Radius(22).
			Background(ui.RGBA(20, 18, 16, 0.72))
		shown := bar.AnimateTo("shown", map[bool]float32{true: 1, false: 0}[controls], ui.Animation{Duration: 250e6, NonMotion: true})
		bar.Opacity(shown)
		if shown < 0.01 {
			bar.Hidden(true)
		}
		bar.Children(func() {
			viewerButton(c, icLeft, "Previous", func() { a.randomBack(c.Now()) })
			pause, label := icPause, "Pause"
			if s.paused {
				pause, label = icPlay, "Play"
			}
			viewerButton(c, pause, label, func() { a.randomPause(c.Now()) })
			viewerButton(c, icRight, "Next", func() { s.advance(a, c.Now()) })
			viewerButton(c, icExternal, "Open artwork", func() {
				a.router.Push("/artwork/" + strconv.Itoa(art.ID))
			})
			if a.immersive {
				viewerButton(c, icShrink, "Exit full screen", func() { a.setImmersive(false) })
			} else {
				viewerButton(c, icExpand, "Full screen", func() { a.setImmersive(true) })
			}
		})
	})
	c.OnWindowShortcut(0, ui.KeySpace, func() { a.randomPause(c.Now()) })
	c.OnWindowShortcut(0, ui.KeyRight, func() { s.advance(a, c.Now()) })
	c.OnWindowShortcut(0, ui.KeyLeft, func() { a.randomBack(c.Now()) })
	c.OnWindowShortcut(0, ui.KeyF, func() { a.setImmersive(!a.immersive) })
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

// randomBack returns to the artwork before, if there is one.
func (a *app) randomBack(now time.Time) {
	s := a.show
	if s == nil || s.prev == nil {
		return
	}
	s.next, s.cur, s.prev = s.cur, s.prev, nil
	s.since = now
	s.lastMove = now
}

func (a *app) randomPause(now time.Time) {
	s := a.show
	if s.paused {
		s.since = now // a full period again
	}
	s.paused = !s.paused
}

// slide is one artwork's picture across the whole window, fading in or out
// and drifting slowly, over its own blurred tone.
func (a *app) slide(c *ui.Context, art *Artwork, on bool, age time.Duration) {
	pic := a.img.get(randomPicture(art), 2400)
	layer := ui.Box(c.Key(art.ID)).Absolute().Top(0).Left(0).Right(0).Bottom(0)
	target := float32(0)
	if on {
		target = 1
	}
	fade := layer.AnimateTo("fade", target, ui.Animation{Duration: 1200e6, NonMotion: true})
	if fade < 0.01 {
		layer.Hidden(true)
		return
	}
	layer.Opacity(fade).Children(func() {
		if pic == nil || pic.bmp == nil {
			return
		}
		ui.Image(c, pic.bmp).Absolute().Top(-60).Left(-60).Right(-60).Bottom(-60).Fit(ui.Cover).Blur(60).Opacity(0.4)
		drift := float32(0)
		if !c.Preferences().ReduceMotion {
			drift = float32(min(age.Seconds()/randomPeriod.Seconds(), 1.5))
		}
		zoom := 1 + 0.05*drift
		ui.Box(c.Key("picture")).Absolute().Top(48).Left(48).Right(48).Bottom(120).Label(cleanTitle(art.Title)).
			Draw(func(p *ui.Painter, frame ui.Rect) {
				iw, ih := ui.ImageSize(pic.bmp)
				fit := min(frame.W/iw, frame.H/ih) * zoom
				w, h := iw*fit, ih*fit
				p.ImageRegion(pic.bmp, ui.Rect{}, ui.Rect{X: frame.X + (frame.W-w)/2, Y: frame.Y + (frame.H-h)/2, W: w, H: h}, ui.ImageOptions{})
			})
	})
	if on && !c.Preferences().ReduceMotion && !a.show.paused {
		c.AnimationFrame()
	}
}
