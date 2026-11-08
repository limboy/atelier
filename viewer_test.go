package main

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.01 }

func TestViewerZoomKeepsPointUnderPointer(t *testing.T) {
	v := &viewer{zoom: 1}
	v.layout(1000, 800, 1)
	// The picture's point under (600, 300) before zooming stays there.
	img := func(x, y float32) (float32, float32) {
		return (x - v.w/2 - v.tx) / v.zoom, (y - v.cy - v.ty) / v.zoom
	}
	ux, uy := img(600, 300)
	v.zoomAt(2, 600, 300)
	if gx, gy := img(600, 300); !near(gx, ux) || !near(gy, uy) {
		t.Fatalf("point moved from (%v, %v) to (%v, %v)", ux, uy, gx, gy)
	}
	if !v.hires {
		t.Fatal("zooming in did not ask for the full resolution picture")
	}
}

func TestViewerPanStaysOverPicture(t *testing.T) {
	v := &viewer{zoom: 1}
	v.layout(1000, 800, 2) // fitted 928 × 464, centered at y 376
	v.zoomAt(1.5, 500, 376)
	v.pan(5000, 5000)
	// 1392 DIPs wide covers the frame; 696 high stays whole in it.
	if !near(v.tx, 1392/2-500) {
		t.Fatalf("tx = %v, want the left edge at the frame's", v.tx)
	}
	if v.ty != 0 {
		t.Fatalf("ty = %v, want the picture shorter than the frame left in place", v.ty)
	}
	v.zoomAt(1/1.5, 500, 400)
	if v.zoom != 1 || v.tx != 0 || v.ty != 0 {
		t.Fatalf("zooming out did not fit the picture: %+v", v)
	}
}

func TestViewerGlidesAndFlings(t *testing.T) {
	v := &viewer{zoom: 1}
	v.layout(1000, 800, 1)
	v.step(1.0/60, false)
	v.zoomAt(3, 500, 376)
	if !v.step(1.0/60, false) || v.sz <= 1 || v.sz >= 3 {
		t.Fatalf("zoom jumped instead of gliding: shown %v", v.sz)
	}
	for range 120 {
		v.step(1.0/60, false)
	}
	if v.sz != 3 {
		t.Fatalf("glide ended at %v, want 3", v.sz)
	}
	v.vx = 2000
	v.step(1.0/60, false)
	if v.tx <= 0 || v.vx >= 2000 {
		t.Fatalf("fling did not move and slow: tx %v vx %v", v.tx, v.vx)
	}
	for range 600 {
		v.step(1.0/60, false)
	}
	if v.vx != 0 || v.tx > 3*v.fw/2-500+0.01 {
		t.Fatalf("fling did not stop at the edge: tx %v vx %v", v.tx, v.vx)
	}
	v.zoomAt(0.5, 500, 400)
	if v.step(1.0/60, true) || v.sz != v.zoom {
		t.Fatal("Reduce Motion glided")
	}
}

func TestCultureAssetViewport(t *testing.T) {
	link := "https://artsandculture.google.com/asset/almond-blossom-vincent-van-gogh/dAFXSL9sZ1ulDw?ms=%7B%22x%22%3A0.5309002745693642%2C%22y%22%3A0.7977822580645162%2C%22z%22%3A11%2C%22size%22%3A%7B%22width%22%3A0.4740363172985027%2C%22height%22%3A0.4044354838709677%7D%7D"
	p, ok := cultureAssetViewport(link)
	if !ok || !near(p.x, 0.5309) || !near(p.y, 0.7978) || !near(p.w, 0.474) || !near(p.h, 0.4044) {
		t.Fatalf("viewport = %+v, %v", p, ok)
	}
	for _, raw := range []string{
		"https://artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw",
		`https://artsandculture.google.com/asset/x/GgHsT2RumWxbtw?ms={"x":2,"y":0.5,"size":{"width":0.5,"height":0.5}}`,
		`https://artsandculture.google.com/asset/x/GgHsT2RumWxbtw?ms={"x":0.5,"y":0.5,"size":{"width":1,"height":1}}`,
		`https://example.com/asset/x/GgHsT2RumWxbtw?ms={"x":0.5,"y":0.5,"size":{"width":0.5,"height":0.5}}`,
	} {
		if p, ok := cultureAssetViewport(raw); ok {
			t.Errorf("%s: viewport %+v, want none", raw, p)
		}
	}
}

func TestSharedLinkOpensZoomedWithOverview(t *testing.T) {
	id := "dAFXSL9sZ1ulDw"
	link := `https://artsandculture.google.com/asset/almond-blossom/` + id + `?ms={"x":0.5,"y":0.8,"size":{"width":0.4,"height":0.4}}`
	a := &app{router: ui.NewRouter("/home"), img: &images{m: map[picKey]*picture{}}, assets: map[string]*assetState{}}
	a.openCultureArtwork(cultureItem{Path: link})
	s := a.assets[id]
	s.art = &cultureArtwork{ID: id, Title: "Almond Blossom", Image: "https://lh3.googleusercontent.com/ci/almond"}
	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1200, 948)))
	for _, size := range []int{1800, 4096} {
		a.img.m[picKey{cultureImage(s.art.Image, size), map[int]int{1800: 1800, 4096: 5000}[size]}] = &picture{bmp: bmp, w: 1200, h: 948}
	}
	view := ui.NewTester(a.view, 1280, 840)
	view.Advance(time.Second)
	if a.router.Path() != "/asset/"+id || s.viewer.pending != nil || s.viewer.zoom <= 1 {
		t.Fatalf("shared region not shown: path %s, viewer %+v", a.router.Path(), s.viewer)
	}
	if u0, w0, u1, w1 := s.viewer.visible(); u0 > 0.5 || u1 < 0.5 || w0 > 0.8 || w1 < 0.8 {
		t.Fatalf("view (%v, %v)–(%v, %v) misses the shared region's center", u0, w0, u1, w1)
	}
	if _, ok := view.Find("Overview"); !ok {
		t.Fatal("zoomed in without an overview")
	}
	if s.viewer.max != 4 {
		t.Fatalf("zoom limit %v for a 1200 pixel picture, want 4", s.viewer.max)
	}
	if err := view.Click("Fit to window"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if _, ok := view.Find("Overview"); ok {
		t.Fatal("overview stayed after fitting the picture")
	}
}
