package main

import (
	"image"
	"image/color"
	"image/draw"
	"strconv"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestHeroZoomsFromFirstVisibleFrame(t *testing.T) {
	for _, delayed := range []bool{false, true} {
		name := "cached image"
		if delayed {
			name = "downloaded image"
		}
		t.Run(name, func(t *testing.T) {
			a, pictures, bitmap := heroFixture(1)
			if !delayed {
				pictures[0].bmp = bitmap
			}
			view := ui.NewTester(func(c *ui.Context) { a.hero(c) }, 1280, 440)
			if delayed {
				view.Advance(3 * time.Second)
				pictures[0].bmp = bitmap
				view.Frame()
			}
			start := heroPictureBounds(t, view.Image())
			view.Advance(4 * time.Second)
			grown := heroPictureBounds(t, view.Image())
			assertHeroGrowsInPlace(t, start, grown)
		})
	}
}

func TestHeroZoomRestartsWhenSlideReturns(t *testing.T) {
	a, pictures, bitmap := heroFixture(2)
	for _, pic := range pictures {
		pic.bmp = bitmap
	}
	view := ui.NewTester(func(c *ui.Context) { a.hero(c) }, 1280, 440)
	view.Advance(4 * time.Second)
	if err := view.Click("Show 2"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if err := view.Click("Show 1"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	start := heroPictureBounds(t, view.Image())
	view.Advance(4 * time.Second)
	assertHeroGrowsInPlace(t, start, heroPictureBounds(t, view.Image()))
}

func TestHeroRespectsReduceMotion(t *testing.T) {
	a, pictures, bitmap := heroFixture(1)
	pictures[0].bmp = bitmap
	view := ui.NewTesterWith(func(c *ui.Context) { a.hero(c) }, 1280, 440,
		ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: true}})
	start := heroPictureBounds(t, view.Image())
	view.Advance(4 * time.Second)
	if later := heroPictureBounds(t, view.Image()); later != start {
		t.Fatalf("artwork moved with Reduce Motion: first %v, later %v", start, later)
	}
}

func heroFixture(count int) (*app, []*picture, *ui.Bitmap) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{G: 255, A: 255}), image.Point{}, draw.Src)
	bitmap := ui.NewBitmap(img)
	f := &feed{done: true}
	a := &app{
		feeds:     map[string]*feed{heroQuery.Key(): f},
		img:       &images{m: map[picKey]*picture{}},
		heroSince: time.Now(),
	}
	var pictures []*picture
	for i := range count {
		art := &Artwork{ID: i + 1, Title: "Featured painting", ImageSmall: "image-" + strconv.Itoa(i)}
		pic := &picture{w: 100, h: 200}
		f.items = append(f.items, art)
		a.img.m[picKey{art.Thumb(), 1600}] = pic
		pictures = append(pictures, pic)
	}
	return a, pictures, bitmap
}

// Only the sharp foreground reaches full green; its blurred backdrop is dimmed.
func heroPictureBounds(t *testing.T, img *image.RGBA) image.Rectangle {
	t.Helper()
	var bounds image.Rectangle
	for y := range img.Bounds().Dy() {
		for x := 900; x < img.Bounds().Dx(); x++ {
			pixel := img.RGBAAt(x, y)
			if pixel.G >= 230 && pixel.R < 10 && pixel.B < 10 {
				bounds = bounds.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if bounds.Empty() {
		t.Fatal("the featured artwork is not visible")
	}
	return bounds
}

func assertHeroGrowsInPlace(t *testing.T, start, grown image.Rectangle) {
	t.Helper()
	if grown.Dx() <= start.Dx() || grown.Dy() <= start.Dy() {
		t.Fatalf("artwork did not slowly enlarge: first %v, later %v", start, grown)
	}
	centerDrift := (grown.Min.Add(grown.Max)).Sub(start.Min.Add(start.Max))
	if centerDrift.X < -2 || centerDrift.X > 2 || centerDrift.Y < -2 || centerDrift.Y > 2 {
		t.Fatalf("artwork drifted while zooming: first %v, later %v", start, grown)
	}
}
