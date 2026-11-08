package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// story is a row of the home page: a theme and the search that fills it.
type story struct {
	kicker, title string
	q             Query
}

var heroQuery = Query{Q: "painting", Department: 11, Highlight: true}

var stories = []story{
	{"Masterpieces", "Highlights of European painting", Query{Q: "oil on canvas", Department: 11, Highlight: true}},
	{"Movement", "The light of Impressionism", Query{Q: "impressionism", Department: 11}},
	{"Artist", "Vincent van Gogh", Query{Q: "Vincent van Gogh", Artist: true}},
	{"Printmaking", "The floating world: Japanese woodblock prints", Query{Q: "ukiyo-e", Department: 6}},
	{"Civilization", "Treasures of Ancient Egypt", Query{Q: "*", Department: 10, Highlight: true}},
	{"Artist", "Rembrandt van Rijn", Query{Q: "Rembrandt", Artist: true}},
	{"Craft", "Armor and the art of the blade", Query{Q: "*", Department: 4, Highlight: true}},
	{"Nature", "Gardens, flowers and still lifes", Query{Q: "flowers still life", Department: 11}},
	{"Antiquity", "Gods and heroes of Greece and Rome", Query{Q: "*", Department: 13, Highlight: true}},
	{"Photography", "Early photography", Query{Q: "albumen silver print", Department: 19, Highlight: true}},
	{"Artist", "Katsushika Hokusai", Query{Q: "Hokusai", Artist: true}},
	{"Design", "Islamic art and ornament", Query{Q: "*", Department: 14, Highlight: true}},
}

func (a *app) home(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(52).Padding(28, 0, 60).Children(func() {
			a.hero(c)
			a.artOfTheDay(c)
			for i, s := range stories {
				// Rows load as they come near, not all as the app opens.
				row := ui.Column(c.Key(i)).Gap(18)
				near := row.InViewMargin(500) || a.feeds[s.q.Key()] != nil
				row.Children(func() {
					a.sectionHeader(c, s.kicker, s.title, collectionPath(s.title, s.kicker, s.q))
					f := &feed{}
					if near {
						f = a.feed(s.q, 12)
					}
					a.strip(c, f, 230, 12)
				})
			}
			a.homeFooter(c)
		})
	})
}

// hero is a carousel of highlights across the width of the page.
func (a *app) hero(c *ui.Context) {
	f := a.feed(heroQuery, 10)
	items := f.items
	if len(items) > 8 {
		items = items[:8]
	}
	const period = 8 * time.Second
	if len(items) > 1 {
		if since := c.Now().Sub(a.heroSince); since >= period {
			a.heroAt = (a.heroAt + 1) % len(items)
			a.heroSince = c.Now()
		} else {
			c.After(period - since)
		}
	}
	ui.Box(c).Height(440).Margin(0, 40).Radius(14).Clip().Background(night).Children(func() {
		if len(items) == 0 {
			ui.Box(c).Fill().Background(ui.Hex("#2a2622")).Opacity(0.5 + 0.5*pulse(c))
			return
		}
		a.heroAt %= len(items)
		for i, art := range items {
			pic := a.img.get(art.Thumb(), 1600)
			layer := ui.Box(c.Key(art.ID)).Absolute().Top(0).Left(0).Right(0).Bottom(0)
			on := float32(0)
			if i == a.heroAt {
				on = 1
			}
			fade := layer.AnimateTo("fade", on, ui.Animation{Duration: 900e6, NonMotion: true})
			if fade < 0.01 {
				layer.Hidden(true)
				continue
			}
			layer.Opacity(fade).Children(func() {
				if pic != nil && pic.bmp != nil {
					// The picture blurred behind fills the frame, the
					// picture itself sits on the right, whole.
					ui.Image(c, pic.bmp).Absolute().Top(-40).Left(-40).Right(-40).Bottom(-40).Fit(ui.Cover).Blur(40).Opacity(0.55)
					image := ui.Box(c.Key("picture")).Absolute().Top(28).Bottom(76).Right(48).WidthPercent(52).
						Label(cleanTitle(art.Title))
					active := ui.Local(image, "active", func() bool { return false })
					if on == 1 && !*active {
						// Animate starts at its first target; seed the unzoomed
						// size when this picture becomes visible, even after loading.
						image.Jump("ken", 0)
					}
					*active = on == 1
					zoom := 1 + 0.04*image.AnimateWith("ken", 1, period+time.Second, ui.Linear)
					image.Draw(func(p *ui.Painter, frame ui.Rect) {
						iw, ih := ui.ImageSize(pic.bmp)
						fit := min(frame.W/iw, frame.H/ih)
						w, h := iw*fit, ih*fit
						// Fit on the right, then enlarge around the artwork's
						// own center rather than the frame's empty space.
						cx, cy := frame.X+frame.W-w/2, frame.Y+frame.H/2
						w, h = w*zoom, h*zoom
						p.ImageRegion(pic.bmp, ui.Rect{}, ui.Rect{X: cx - w/2, Y: cy - h/2, W: w, H: h}, ui.ImageOptions{})
					})
				}
				ui.Box(c).Absolute().Top(0).Left(0).Bottom(0).WidthPercent(70).
					Gradient(ui.RGBA(14, 12, 10, 0.92), ui.RGBA(14, 12, 10, 0), 90)
			})
		}
		art := items[a.heroAt]
		ui.Column(c.Key("caption")).Absolute().Left(48).Bottom(48).WidthPercent(42).Gap(12).Children(func() {
			ui.Text(c, "FEATURED · "+art.Department).FontSize(11).FontWeight(600).LetterSpacing(1.6).TextColor(ui.Hex("#f0c9a8"))
			ui.Text(c, cleanTitle(art.Title)).Font(serif).FontSize(38).LineHeight(1.1).TextColor(paper).MaxLines(3).Balance()
			ui.Text(c, art.Byline()+dateSuffix(art.Date)).FontSize(15).TextColor(ui.Hex("#d9d2c7"))
			ui.Row(c).Gap(10).Padding(14, 0, 0).Children(func() {
				b := ui.ButtonBase(c.Key("open")).Padding(10, 18).Radius(20).Background(paper).TextColor(ink).Cursor(ui.CursorPointer)
				if b.Hovered() {
					b.Background(ui.Hex("#ffffff"))
				}
				b.Children(func() { ui.Text(c, "View artwork").FontWeight(600) })
				if b.Clicked() {
					a.router.Push("/artwork/" + strconv.Itoa(art.ID))
				}
			})
		})
		// Dots, and arrows going from one to the next.
		ui.Row(c.Key("dots")).Absolute().Right(48).Bottom(28).Gap(8).AlignItems(ui.Center).Children(func() {
			heroArrow(c, icLeft, "Previous", func() { a.heroAt = (a.heroAt + len(items) - 1) % len(items); a.heroSince = c.Now() })
			for i := range items {
				d := ui.ButtonBase(c.Key(i)).Height(8).Radius(4).Cursor(ui.CursorPointer).Label("Show " + strconv.Itoa(i+1))
				w := float32(8)
				col := ui.RGBA(255, 255, 255, 0.4)
				if i == a.heroAt {
					w, col = 22, paper
				}
				d.Width(d.AnimateTo("w", w, ui.Snappy)).Background(col)
				if d.Clicked() {
					a.heroAt, a.heroSince = i, c.Now()
				}
			}
			heroArrow(c, icRight, "Next", func() { a.heroAt = (a.heroAt + 1) % len(items); a.heroSince = c.Now() })
		})
	})
}

func heroArrow(c *ui.Context, ic *ui.SVG, label string, fn func()) {
	b := ui.ButtonBase(c.Key(label)).Size(32, 32).Radius(16).Label(label).Cursor(ui.CursorPointer).
		Background(ui.RGBA(255, 255, 255, 0.12)).TextColor(paper)
	if b.Hovered() {
		b.Background(ui.RGBA(255, 255, 255, 0.24))
	}
	b.Children(func() { ui.Icon(c, ic).Size(16, 16) }).OnClick(fn)
}

func dateSuffix(d string) string {
	if d == "" {
		return ""
	}
	return " · " + d
}

// artOfTheDay picks a highlight by the date, beside a short card about it.
func (a *app) artOfTheDay(c *ui.Context) {
	f := a.feed(Query{Q: "*", Highlight: true, Department: 11}, 20)
	if len(f.items) == 0 {
		return
	}
	day := c.Now().YearDay() + c.Now().Year()*366
	art := f.items[day%len(f.items)]
	pic := a.img.get(art.Thumb(), 1200)
	ui.Row(c).Margin(0, 40).Gap(40).AlignItems(ui.Center).Padding(28).Radius(14).Background(wash).Children(func() {
		frame := ui.ButtonBase(c.Key("pic")).Width(360).Height(300).Radius(6).Clip().Background(wash).Cursor(ui.CursorPointer).Label(cleanTitle(art.Title))
		frame.Children(func() {
			if pic != nil && pic.bmp != nil {
				ui.Image(c, pic.bmp).Fill().Fit(ui.Contain)
			}
		})
		if frame.Clicked() {
			a.router.Push("/artwork/" + strconv.Itoa(art.ID))
		}
		ui.Column(c).Grow(1).Gap(10).Children(func() {
			ui.Text(c, "ART OF THE DAY · "+c.Now().Format("January 2")).FontSize(11).FontWeight(600).LetterSpacing(1.4).TextColor(accent)
			ui.Text(c, cleanTitle(art.Title)).Font(serif).FontSize(30).LineHeight(1.15).Balance()
			ui.Text(c, art.Byline()+dateSuffix(art.Date)).FontSize(15).TextColor(inkSoft)
			if art.Medium != "" {
				ui.Text(c, art.Medium).TextColor(inkSoft).MaxLines(2)
			}
			ui.Row(c).Gap(10).Padding(12, 0, 0).Children(func() {
				if ui.PrimaryButton(c, "Take a closer look").Clicked() {
					a.router.Push("/artwork/" + strconv.Itoa(art.ID))
				}
				a.favoriteButton(c, art.ID)
			})
		})
	})
}

func (a *app) homeFooter(c *ui.Context) {
	ui.Column(c).Padding(20, 40).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Divider(c).Width(80).Margin(0, 0, 16)
		ui.Text(c, "Works from The Metropolitan Museum of Art’s Open Access collection, in the public domain.").
			FontSize(12).TextColor(inkFaint).TextAlign(ui.Center)
	})
}
