package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func testPyramid() *culturePyramid {
	p := &culturePyramid{base: "https://lh3.googleusercontent.com/ci/test", tileWidth: 512, tileHeight: 512}
	for n := 1; n <= 16; n *= 2 {
		p.levels = append(p.levels, tileLevel{n * 512, n * 512, n, n})
	}
	return p
}

func TestTilesChooseDetailForDisplayDensity(t *testing.T) {
	p := testPyramid()
	v := &viewer{zoom: 4}
	v.layout(800, 800, 1)
	v.snap()
	for _, tc := range []struct {
		scale float32
		level int
	}{{1, 3}, {2, 4}} {
		coords := p.plan(v, tc.scale)
		if len(coords) == 0 {
			t.Fatal("no visible tiles requested")
		}
		for _, c := range coords {
			if c.z != tc.level {
				t.Fatalf("density %v chose level %d, want %d", tc.scale, c.z, tc.level)
			}
			if w, h := p.tileSize(c); w <= 0 || h <= 0 {
				t.Fatalf("invalid tile requested: %+v", c)
			}
		}
		if len(coords)*p.tileWidth*p.tileHeight*6 > tileCacheBytes {
			t.Fatal("viewport exceeds tile memory budget")
		}
	}
	// A huge display must select a coarser level instead of allocating an
	// unbounded tile list or exceeding the decoded cache's capacity.
	v.layout(30000, 20000, 1)
	v.snap()
	if coords := p.plan(v, 3); len(coords)*512*512*6 > tileCacheBytes {
		t.Fatal("large display exceeds memory budget")
	}
}

func TestTileRangeAtExactEdges(t *testing.T) {
	p := testPyramid()
	v := &viewer{w: 512, h: 512, fw: 1024, fh: 1024, cy: 256, sz: 1, sx: 256, sy: 256}
	if got := p.tileRange(1, v); got != image.Rect(0, 0, 1, 1) {
		t.Fatalf("tile edge requested extra tiles: %v", got)
	}
}

func TestTilesCancelObsoleteViewportAndNavigation(t *testing.T) {
	g := NewCulture(t.TempDir())
	entered := make(chan *http.Request, 32)
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		entered <- r
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	a := &app{culture: g}
	s := newArtworkTiles(a)
	posted := make(chan func(), 32)
	s.post = func(fn func()) { posted <- fn }
	p := testPyramid()
	tiles := &tileView{pyramid: p}
	s.active = tiles
	v := &viewer{zoom: 8, tiles: tiles}
	v.layout(800, 800, 1)
	v.snap()
	s.request(v, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	requests := make([]*http.Request, 0, tileDownloads)
	for range tileDownloads {
		select {
		case r := <-entered:
			requests = append(requests, r)
		case <-ctx.Done():
			t.Fatal("tile workers did not start")
		}
	}
	if len(s.jobs) != tileDownloads {
		t.Fatal("download concurrency is not bounded")
	}
	v.pan(100000, 100000)
	v.snap()
	s.request(v, 1)
	for _, r := range requests {
		select {
		case <-r.Context().Done():
		case <-ctx.Done():
			t.Fatal("offscreen request was not canceled")
		}
	}
	s.deactivate()
	for len(s.jobs) > 0 {
		select {
		case fn := <-posted:
			fn()
		case <-ctx.Done():
			t.Fatal("leaving the artwork did not finish canceled jobs")
		}
	}
	if len(s.cache) != 0 || len(s.failed) != 0 || len(s.pending) != 0 {
		t.Fatal("canceled work published pixels, errors or more downloads")
	}
}

func TestTileCacheEvictsOldDetailAndKeepsVisibleTiles(t *testing.T) {
	s := newArtworkTiles(&app{})
	s.wanted = map[tileKey]bool{}
	const bytes = 512 * 512 * 6
	for i := range 60 {
		key := tileKey{base: "scan", tileCoord: tileCoord{x: i}}
		s.cache[key] = &detailTile{bytes: bytes, used: uint64(i)}
		s.bytes += bytes
		if i == 0 {
			s.wanted[key] = true
		}
	}
	s.trim()
	if s.bytes > tileCacheBytes || s.cache[tileKey{base: "scan"}] == nil {
		t.Fatal("cache exceeded its budget or evicted visible detail")
	}
	if s.cache[tileKey{base: "scan", tileCoord: tileCoord{x: 1}}] != nil {
		t.Fatal("oldest unused tile was retained")
	}
	s.clear()
	if len(s.cache) != 0 || s.bytes != 0 {
		t.Fatal("clearing the cache retained decoded tiles")
	}
}

func TestTileRenderingCoversFractionalEdgesWithoutSeams(t *testing.T) {
	p := &culturePyramid{base: "scan", tileWidth: 16, tileHeight: 16, levels: []tileLevel{{30, 24, 2, 2}}}
	tiles := &tileView{pyramid: p}
	s := newArtworkTiles(&app{})
	s.active = tiles
	for y := range 2 {
		for x := range 2 {
			c := tileCoord{0, x, y}
			w, h := p.tileSize(c)
			m := image.NewRGBA(image.Rect(0, 0, w, h))
			for py := range h {
				for px := range w {
					m.SetRGBA(px, py, color.RGBA{R: 30 + uint8(x)*100, G: 40 + uint8(y)*100, B: 70, A: 255})
				}
			}
			s.cache[tileKey{p.base, c}] = &detailTile{bmp: ui.NewBitmap(m), w: w, h: h}
		}
	}
	v := &viewer{zoom: 1.137, tiles: tiles}
	v.layout(300, 300, 1.25)
	v.snap()
	v.sx, v.sy = 0.37, 0.19
	view := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.Hex("#ff00ff")).Draw(func(p *ui.Painter, r ui.Rect) { s.draw(p, r, v) })
	}, 300, 300)
	m := view.Image()
	box := v.pictureRect(ui.Rect{})
	for y := int(box.Y) + 2; y < int(box.Y+box.H)-2; y++ {
		for x := int(box.X) + 2; x < int(box.X+box.W)-2; x++ {
			c := m.RGBAAt(x, y)
			if c.B != 70 || c.A != 255 {
				t.Fatalf("tile gap at (%d, %d): %v", x, y, c)
			}
		}
	}
}

func TestTiledViewerUsesSourceResolutionForSharedRegion(t *testing.T) {
	id := "deep-artwork"
	p := testPyramid()
	p.levels = append(p.levels, tileLevel{32768, 32768, 64, 64})
	state := &assetState{art: &cultureArtwork{ID: id, Title: "A detailed painting", Image: "https://lh3.googleusercontent.com/ci/test"}}
	state.viewer.tiles = &tileView{id: id, pyramid: p}
	state.viewer.pending = &viewport{0.8, 0.7, 0.01, 0.01}
	a := &app{router: ui.NewRouter("/asset/" + id), assets: map[string]*assetState{id: state}, img: &images{m: map[picKey]*picture{}}}
	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1200, 1200)))
	a.img.m[picKey{cultureImage(state.art.Image, 1800), 1800}] = &picture{bmp: bmp, w: 1200, h: 1200}
	view := ui.NewTester(a.view, 1280, 840)
	view.Advance(time.Second)
	if state.viewer.max <= maxZoom || state.viewer.zoom <= maxZoom || state.viewer.pending != nil {
		t.Fatalf("tile metadata did not enable a deep shared region: %+v", state.viewer)
	}
	x0, y0, x1, y1 := state.viewer.visible()
	if x0 > 0.8 || x1 < 0.8 || y0 > 0.7 || y1 < 0.7 {
		t.Fatalf("shared region moved: (%v, %v)–(%v, %v)", x0, y0, x1, y1)
	}
	if _, ok := view.Find("Overview"); !ok {
		t.Fatal("deep zoom lost the overview")
	}
}

func TestTiledArtworkStreamsDetailOverPreview(t *testing.T) {
	g := NewCulture(t.TempDir())
	detail := image.NewRGBA(image.Rect(0, 0, 256, 256))
	draw.Draw(detail, detail.Bounds(), image.NewUniform(color.RGBA{R: 220, G: 20, B: 10, A: 255}), image.Point{}, draw.Src)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, detail, nil); err != nil {
		t.Fatal(err)
	}
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(encoded.Bytes()))}, nil
	})
	id := "streaming-artwork"
	p := &culturePyramid{base: "https://lh3.googleusercontent.com/ci/stream", tileWidth: 256, tileHeight: 256, levels: []tileLevel{{512, 512, 2, 2}}}
	state := &assetState{art: &cultureArtwork{ID: id, Title: "Streaming painting", Image: p.base}}
	state.viewer = viewer{zoom: 2, tiles: &tileView{id: id, pyramid: p}}
	a := &app{culture: g, router: ui.NewRouter("/asset/" + id), assets: map[string]*assetState{id: state}, img: &images{m: map[picKey]*picture{}}}
	preview := image.NewRGBA(image.Rect(0, 0, 128, 128))
	draw.Draw(preview, preview.Bounds(), image.NewUniform(color.RGBA{B: 180, A: 255}), image.Point{}, draw.Src)
	a.img.m[picKey{cultureImage(p.base, 1800), 1800}] = &picture{bmp: ui.NewBitmap(preview), w: 128, h: 128}
	a.tiles = newArtworkTiles(a)
	posted := make(chan func(), 16)
	a.tiles.post = func(fn func()) { posted <- fn }
	view := ui.NewTester(a.view, 1280, 840)
	view.Advance(time.Second)
	x, y := int(state.viewer.w/2), int(state.viewer.cy)+int(topBarHeight)
	if c := view.Image().RGBAAt(x, y); c.B < 150 || c.R > 30 {
		t.Fatalf("preview was not visible before tiles arrived: %v at (%d, %d), viewer %+v", c, x, y, state.viewer)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for len(a.tiles.cache) != 4 || len(a.tiles.jobs) != 0 {
		select {
		case fn := <-posted:
			fn()
			view.Frame()
		case <-ctx.Done():
			t.Fatal("detail did not finish streaming")
		}
	}
	if c := view.Image().RGBAAt(x, y); c.R < 200 || c.B > 30 {
		t.Fatalf("detail did not replace preview pixels: %v", c)
	}
	if _, ok := a.img.m[picKey{cultureImage(p.base, 4096), 5000}]; ok {
		t.Fatal("tiled viewer unnecessarily loaded a full-image rendition")
	}
}

func TestTileStatusIgnoresMissingPyramidAndPrefetchRing(t *testing.T) {
	s := newArtworkTiles(&app{})
	tv := &tileView{err: fmt.Errorf("%w: no tile information", errNoTiles)}
	s.activate(tv)
	if loading, failed := s.status(tv); loading || failed {
		t.Fatalf("artwork without tiles reported loading %v, failed %v", loading, failed)
	}
	in, ring := tileKey{"b", tileCoord{1, 0, 0}}, tileKey{"b", tileCoord{1, 1, 0}}
	s.wanted = map[tileKey]bool{in: true, ring: true}
	s.visible = map[tileKey]bool{in: true}
	s.cache[in] = &detailTile{}
	s.failed[ring] = true
	if loading, failed := s.status(tv); loading || failed {
		t.Fatalf("prefetch ring reported loading %v, failed %v", loading, failed)
	}
	s.failed[in], s.cache = true, map[tileKey]*detailTile{}
	if _, failed := s.status(tv); !failed {
		t.Fatal("a visible tile failed without an error")
	}
}

func TestTilesPlanFromWhereTheZoomIsHeaded(t *testing.T) {
	s := newArtworkTiles(&app{})
	p := testPyramid()
	v := &viewer{zoom: 1, tiles: &tileView{pyramid: p}}
	v.layout(800, 800, 1)
	v.snap()
	v.zoomAt(8, 400, 376) // the frame still shows zoom 1
	s.request(v, 1)
	target := *v
	target.snap()
	want := p.plan(&target, 1)[0].z
	if len(s.wanted) == 0 {
		t.Fatal("no tiles wanted")
	}
	for key := range s.wanted {
		if key.z != want {
			t.Fatalf("glide asked for level %d, want the target's %d", key.z, want)
		}
	}
}

func TestSharedRegionYieldsToTheUserWhileTilesLoad(t *testing.T) {
	id := "slow-artwork"
	state := &assetState{art: &cultureArtwork{ID: id, Title: "A slow painting", Image: "https://lh3.googleusercontent.com/ci/slow"}}
	state.viewer.tiles = &tileView{id: id, loading: true}
	state.viewer.pending = &viewport{0.3, 0.3, 0.2, 0.2}
	a := &app{router: ui.NewRouter("/asset/" + id), assets: map[string]*assetState{id: state}, img: &images{m: map[picKey]*picture{}}}
	bmp := ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 1200, 1200)))
	for _, size := range []int{1800, 4096} {
		a.img.m[picKey{cultureImage(state.art.Image, size), map[int]int{1800: 1800, 4096: 5000}[size]}] = &picture{bmp: bmp, w: 1200, h: 1200}
	}
	view := ui.NewTester(a.view, 1280, 840)
	view.Advance(time.Second)
	v := &state.viewer
	if v.pending == nil || v.zoom <= 1 {
		t.Fatalf("shared region not shown while tiles load: %+v", v)
	}
	if err := view.Click("Fit to window"); err != nil {
		t.Fatal(err)
	}
	view.Advance(time.Second)
	if v.pending != nil || v.zoom != 1 {
		t.Fatalf("shared region overrode fitting the picture: zoom %v, pending %v", v.zoom, v.pending)
	}
}

func TestFailedTilesFallBackToTheLargePicture(t *testing.T) {
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 404, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil))}, nil
	})
	id := "broken-tiles"
	p := &culturePyramid{base: "https://lh3.googleusercontent.com/ci/broken", tileWidth: 256, tileHeight: 256, levels: []tileLevel{{512, 512, 2, 2}}}
	state := &assetState{art: &cultureArtwork{ID: id, Title: "Broken tiles", Image: p.base}}
	state.viewer = viewer{zoom: 2, hires: true, tiles: &tileView{id: id, pyramid: p}}
	a := &app{culture: g, router: ui.NewRouter("/asset/" + id), assets: map[string]*assetState{id: state}, img: &images{m: map[picKey]*picture{}}}
	a.img.m[picKey{cultureImage(p.base, 1800), 1800}] = &picture{bmp: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 128, 128))), w: 128, h: 128}
	a.tiles = newArtworkTiles(a)
	posted := make(chan func(), 16)
	a.tiles.post = func(fn func()) { posted <- fn }
	view := ui.NewTester(a.view, 1280, 840)
	view.Advance(time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for len(a.tiles.failed) == 0 || len(a.tiles.jobs) != 0 {
		select {
		case fn := <-posted:
			fn()
			view.Frame()
		case <-ctx.Done():
			t.Fatal("tiles did not fail")
		}
	}
	view.Frame()
	if _, ok := a.img.m[picKey{cultureImage(p.base, 4096), 5000}]; !ok {
		t.Fatal("failed tiles did not fall back to the large picture")
	}
	if _, ok := view.Find("Retry detail"); !ok {
		t.Fatal("failed tiles offered no retry")
	}
}

func TestReopeningRetriesAnUnreachablePyramidOnly(t *testing.T) {
	id := "GgHsT2RumWxbtw"
	a := &app{router: ui.NewRouter("/home"), assets: map[string]*assetState{}}
	s := &assetState{art: &cultureArtwork{ID: id}}
	a.assets[id] = s
	s.viewer.tiles = &tileView{id: id, err: errors.New("connection reset")}
	a.openCultureArtwork(cultureItem{Path: "/asset/gleaners/" + id})
	if s.viewer.tiles.err != nil {
		t.Fatal("reopening did not retry the pyramid")
	}
	s.viewer.tiles.err = fmt.Errorf("%w: no tile information", errNoTiles)
	a.openCultureArtwork(cultureItem{Path: "/asset/gleaners/" + id})
	if s.viewer.tiles.err == nil {
		t.Fatal("reopening refetches a pyramid the artwork does not have")
	}
}
