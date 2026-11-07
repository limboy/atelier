package main

import (
	"github.com/egoist/mygo/ui"
)

// topic is a tile of the explore page.
type topic struct {
	name, sub string
	q         Query
}

var exploreTabs = []struct {
	name   string
	topics []topic
}{
	{"Artists", []topic{
		{"Claude Monet", "French, 1840–1926", Query{Q: "Claude Monet", Artist: true}},
		{"Vincent van Gogh", "Dutch, 1853–1890", Query{Q: "Vincent van Gogh", Artist: true}},
		{"Rembrandt", "Dutch, 1606–1669", Query{Q: "Rembrandt", Artist: true}},
		{"Johannes Vermeer", "Dutch, 1632–1675", Query{Q: "Johannes Vermeer", Artist: true}},
		{"Edgar Degas", "French, 1834–1917", Query{Q: "Edgar Degas", Artist: true}},
		{"Paul Cézanne", "French, 1839–1906", Query{Q: "Paul Cézanne", Artist: true}},
		{"Katsushika Hokusai", "Japanese, 1760–1849", Query{Q: "Hokusai", Artist: true}},
		{"Utagawa Hiroshige", "Japanese, 1797–1858", Query{Q: "Hiroshige", Artist: true}},
		{"Albrecht Dürer", "German, 1471–1528", Query{Q: "Albrecht Dürer", Artist: true}},
		{"Francisco de Goya", "Spanish, 1746–1828", Query{Q: "Goya", Artist: true}},
		{"El Greco", "Greek, 1541–1614", Query{Q: "El Greco", Artist: true}},
		{"John Singer Sargent", "American, 1856–1925", Query{Q: "John Singer Sargent", Artist: true}},
		{"Winslow Homer", "American, 1836–1910", Query{Q: "Winslow Homer", Artist: true}},
		{"Pierre-Auguste Renoir", "French, 1841–1919", Query{Q: "Renoir", Artist: true}},
		{"J. M. W. Turner", "British, 1775–1851", Query{Q: "Turner", Artist: true, Department: 11}},
		{"Mary Cassatt", "American, 1844–1926", Query{Q: "Mary Cassatt", Artist: true}},
	}},
	{"Movements", []topic{
		{"Impressionism", "1860s–1890s", Query{Q: "impressionism"}},
		{"Post-Impressionism", "1880s–1900s", Query{Q: "post-impressionism"}},
		{"Renaissance", "14th–16th century", Query{Q: "renaissance", Department: 11}},
		{"Baroque", "17th century", Query{Q: "baroque"}},
		{"Rococo", "18th century", Query{Q: "rococo"}},
		{"Romanticism", "c. 1800–1850", Query{Q: "romanticism"}},
		{"Realism", "1840s–1880s", Query{Q: "realism", Department: 11}},
		{"Ukiyo-e", "17th–19th century", Query{Q: "ukiyo-e"}},
		{"Hudson River School", "1825–1870", Query{Q: "Hudson River School"}},
		{"Pictorialism", "1885–1915", Query{Q: "pictorialism"}},
		{"Art Nouveau", "1890–1910", Query{Q: "art nouveau"}},
		{"Gothic", "12th–16th century", Query{Q: "gothic", Department: 17}},
	}},
	{"Mediums", []topic{
		{"Oil paint", "On canvas and panel", Query{Q: "oil on canvas", Highlight: true}},
		{"Watercolor", "Washes of pigment", Query{Q: "watercolor"}},
		{"Woodblock print", "Ink and color on paper", Query{Q: "woodblock print"}},
		{"Etching", "Lines bitten in metal", Query{Q: "etching"}},
		{"Bronze", "Cast metal", Query{Q: "bronze", Highlight: true}},
		{"Marble", "Carved stone", Query{Q: "marble", Highlight: true}},
		{"Porcelain", "Fired white clay", Query{Q: "porcelain", Highlight: true}},
		{"Gold", "The precious metal", Query{Q: "gold", Highlight: true}},
		{"Silk", "Woven and painted", Query{Q: "silk", Highlight: true}},
		{"Tempera", "Pigment in egg", Query{Q: "tempera"}},
		{"Photograph", "Light on paper", Query{Q: "photograph", Department: 19}},
		{"Glass", "Blown and cut", Query{Q: "glass", Highlight: true}},
	}},
	{"Collections", []topic{
		{"European Paintings", "", Query{Department: 11, Highlight: true}},
		{"Asian Art", "", Query{Department: 6, Highlight: true}},
		{"Egyptian Art", "", Query{Department: 10, Highlight: true}},
		{"Greek and Roman Art", "", Query{Department: 13, Highlight: true}},
		{"Islamic Art", "", Query{Department: 14, Highlight: true}},
		{"Arms and Armor", "", Query{Department: 4, Highlight: true}},
		{"Medieval Art", "", Query{Department: 17, Highlight: true}},
		{"The American Wing", "", Query{Department: 1, Highlight: true}},
		{"Drawings and Prints", "", Query{Department: 9, Highlight: true}},
		{"Photographs", "", Query{Department: 19, Highlight: true}},
		{"Arts of Africa, Oceania, and the Americas", "", Query{Department: 5, Highlight: true}},
		{"Musical Instruments", "", Query{Department: 18, Highlight: true}},
		{"The Cloisters", "", Query{Department: 7, Highlight: true}},
		{"Ancient West Asian Art", "", Query{Department: 3, Highlight: true}},
		{"European Sculpture and Decorative Arts", "", Query{Department: 12, Highlight: true}},
		{"The Robert Lehman Collection", "", Query{Department: 15, Highlight: true}},
	}},
}

func (a *app) explore(c *ui.Context) {
	tab := exploreTabs[a.exploreTab]
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(36, 40, 60).Gap(26).Children(func() {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Explore").Font(serif).FontSize(40)
				ui.Text(c, "Wander the collection by who made it, how, and where it came from.").FontSize(15).TextColor(inkSoft)
			})
			ui.Row(c).Gap(8).Children(func() {
				for i, t := range exploreTabs {
					chip(c.Key(i), t.name, i == a.exploreTab, func() { a.exploreTab = i })
				}
			})
			w, _ := c.Size()
			cols := max(2, int((w-80+20)/(250+20)))
			ui.Grid(c.Key(tab.name)).Columns(cols).Gap(20).Children(func() {
				for _, t := range tab.topics {
					a.topicTile(c.Key(t.name), t, tab.name)
				}
			})
		})
	})
}

func chip(c *ui.Context, label string, on bool, fn func()) {
	b := ui.ButtonBase(c).Padding(8, 16).Radius(18).Border(1, line).Cursor(ui.CursorPointer).Selected(on)
	switch {
	case on:
		b.Background(ink).BorderColor(ink).TextColor(paper)
	case b.Hovered():
		b.Background(wash).TextColor(ink)
	default:
		b.Background(card).TextColor(inkSoft)
	}
	b.Children(func() { ui.Text(c, label).FontWeight(500) }).OnClick(fn)
}

// topicTile is a topic's name over a picture of one of its works.
func (a *app) topicTile(c *ui.Context, t topic, kicker string) {
	f := a.feed(t.q, 3)
	var pic *picture
	for _, art := range f.items {
		if p := a.img.get(art.Thumb(), 720); p != nil {
			pic = p
			break
		}
	}
	b := ui.ButtonBase(c).Column().AspectRatio(1.05).Radius(10).Clip().Background(ui.Hex("#2a2622")).
		Cursor(ui.CursorPointer).Label(t.name)
	hover := b.AnimateTo("hover", map[bool]float32{true: 1, false: 0}[b.Hovered()], ui.Smooth)
	b.Children(func() {
		if pic != nil && pic.bmp != nil {
			img := ui.Image(c, pic.bmp).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).
				Scale(1+0.05*hover, 1+0.05*hover)
			img.Opacity(img.AnimateTo("in", 1, ui.Animation{Duration: 400e6, NonMotion: true}))
		} else {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.Hex("#3a342e")).Opacity(0.4 + 0.6*pulse(c))
		}
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).HeightPercent(60).
			Gradient(ui.RGBA(10, 8, 6, 0), ui.RGBA(10, 8, 6, 0.85), 180)
		ui.Column(c).Absolute().Left(18).Right(18).Bottom(16).Gap(3).Children(func() {
			ui.Text(c, t.name).Font(serif).FontSize(21).TextColor(paper).MaxLines(2)
			sub := t.sub
			if f.total > 0 {
				if sub != "" {
					sub += " · "
				}
				sub += plural(f.total, "item", "items")
			}
			if sub != "" {
				ui.Text(c, sub).FontSize(12).TextColor(ui.Hex("#d9d2c7"))
			}
		})
	})
	if b.Clicked() {
		a.router.Push(collectionPath(t.name, kicker, t.q))
	}
}
