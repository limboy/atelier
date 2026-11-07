package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"
)

// pageHeader is the title of a page of artworks.
func pageHeader(c *ui.Context, kicker, title, sub string) {
	ui.Column(c).Gap(6).Children(func() {
		if kicker != "" {
			ui.Text(c, kicker).FontSize(11).FontWeight(600).LetterSpacing(1.4).TextColor(accent)
		}
		ui.Text(c, title).Font(serif).FontSize(40).LineHeight(1.1).Balance()
		if sub != "" {
			ui.Text(c, sub).FontSize(14).TextColor(inkSoft)
		}
	})
}

// collectionPage shows a query's artworks in columns, loading more as it
// scrolls.
func (a *app) collectionPage(c *ui.Context, title, kicker string, q Query) {
	f := a.feed(q, 30)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(36, 40, 40).Gap(28).Children(func() {
			sub := ""
			if f.total > 0 {
				sub = plural(f.total, "item", "items")
			} else if f.loading {
				sub = "Gathering works…"
			}
			kick := ""
			if kicker != "" {
				kick = kickerCase(kicker)
			}
			pageHeader(c, kick, title, sub)
			w, _ := c.Size()
			a.masonry(c, q.Key(), f.items, w-80)
			a.feedFooter(c, f)
		})
	})
}

func kickerCase(s string) string {
	out := []rune(s)
	for i, r := range out {
		if r >= 'a' && r <= 'z' {
			out[i] = r - 32
		}
	}
	return string(out)
}

func (a *app) favoritesPage(c *ui.Context) {
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(36, 40, 40).Gap(28).Children(func() {
			n := len(a.favs.ids)
			sub := "Works you keep, saved on this computer."
			if n > 0 {
				sub = plural(n, "work", "works") + " you keep, saved on this computer."
			}
			pageHeader(c, "YOUR GALLERY", "Favorites", sub)
			if n == 0 {
				ui.Column(c).Padding(60, 0).Gap(12).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icHeart).Size(40, 40).TextColor(inkFaint)
					ui.Text(c, "Tap the heart on an artwork to keep it here.").TextColor(inkSoft)
					if ui.PrimaryButton(c, "Start exploring").Clicked() {
						a.router.Push("/explore")
					}
				})
				return
			}
			var items []*Artwork
			for _, id := range a.favs.ids {
				if s := a.artwork(id); s.art != nil && s.art.Thumb() != "" {
					items = append(items, s.art)
				}
			}
			w, _ := c.Size()
			// Removing a favorite reorders the columns from the start.
			key := fmt.Sprint("favorites", a.favs.ids)
			a.masonry(c, key, items, w-80)
		})
	})
}

// eras are the stops of the time machine.
var eras = []struct {
	name     string
	from, to int
}{
	{"Ancient Egypt", -3000, -1000},
	{"Classical antiquity", -800, 400},
	{"Middle Ages", 500, 1400},
	{"Renaissance", 1400, 1600},
	{"Baroque", 1600, 1720},
	{"Enlightenment", 1720, 1800},
	{"Romantic age", 1800, 1860},
	{"Belle Époque", 1860, 1914},
	{"Modern", 1914, 1960},
}

const (
	timeMin  = -3000
	timeMax  = 1950
	timeSpan = 25
)

func yearLabel(y int) string {
	if y < 0 {
		return strconv.Itoa(-y) + " BCE"
	}
	return strconv.Itoa(y)
}

// spanAt is the range of years the time machine shows around y: wider in
// antiquity, where works are dated loosely.
func spanAt(y int) int {
	switch {
	case y < 0:
		return 200
	case y < 1300:
		return 100
	case y < 1700:
		return 40
	}
	return timeSpan
}

func (a *app) timeMachine(c *ui.Context) {
	// The slider settles on a year a moment after it stops moving.
	if y := int(a.timeSlider); y != a.timeYear {
		if d := c.Now().Sub(a.timeTouched); d >= 350*time.Millisecond {
			a.timeYear = y
		} else {
			c.After(350*time.Millisecond - d)
		}
	}
	span := spanAt(a.timeYear)
	q := Query{Q: "*", Highlight: a.timeYear >= 1300, HasDates: true, From: a.timeYear, To: a.timeYear + span}
	f := a.feed(q, 20)
	if a.timeFeed != f {
		// The years passed while dragging still loading: let them go.
		if a.timeFeed != nil && a.timeFeed.loading {
			a.dropFeed(a.timeFeed)
		}
		a.timeFeed = f
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(36, 40, 40).Gap(26).Children(func() {
			era := ""
			for _, e := range eras {
				if a.timeYear >= e.from && a.timeYear < e.to {
					era = e.name
				}
			}
			ui.Row(c).AlignItems(ui.End).Gap(24).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					pageHeader(c, "TIME MACHINE", "Travel through five thousand years", "Drag through time and see what people were making.")
				})
				ui.Column(c).AlignItems(ui.End).Gap(2).Children(func() {
					ui.Text(c, yearLabel(int(a.timeSlider))).Font(serif).FontSize(54).FontFeatures("tnum").TextColor(accent)
					sub := "to " + yearLabel(int(a.timeSlider)+span)
					if era != "" {
						sub = era + " · " + sub
					}
					ui.Text(c, sub).TextColor(inkSoft)
				})
			})
			ui.Column(c).Gap(14).Padding(22, 24).Radius(12).Background(wash).Children(func() {
				s := ui.Slider(c.Key("year"), &a.timeSlider, timeMin, timeMax).Step(10).Label("Year")
				if s.Changed() {
					a.timeTouched = c.Now()
				}
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for i, e := range eras {
						on := a.timeYear >= e.from && a.timeYear < e.to
						chip(c.Key(i), e.name, on, func() {
							a.timeSlider = float64(e.from)
							a.timeYear = e.from
						})
					}
				})
			})
			w, _ := c.Size()
			a.masonry(c, q.Key(), f.items, w-80)
			a.feedFooter(c, f)
		})
	})
}
