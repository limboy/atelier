package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/draw"
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
)

const tileCacheBytes = 64 << 20
const tileDownloads = 4

// A tileView belongs to an artwork. Decoded tiles live in a shared, bounded
// cache instead, so going through many artworks cannot retain whole scans.
type tileView struct {
	id      string
	pyramid *culturePyramid
	loading bool
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
}

type tileKey struct {
	base string
	tileCoord
}

type detailTile struct {
	bmp   *ui.Bitmap
	w, h  int
	used  uint64
	bytes int
}

type tileJob struct{ cancel context.CancelFunc }

// All state is owned by the UI thread. Only fetching and decoding run on
// workers, with completions posted back through the window's Update.
type artworkTiles struct {
	app     *app
	post    func(func())
	active  *tileView
	cache   map[tileKey]*detailTile
	jobs    map[tileKey]*tileJob
	wanted  map[tileKey]bool
	visible map[tileKey]bool // the wanted tiles inside the view, not the prefetch ring
	failed  map[tileKey]bool
	pending []tileCoord
	bytes   int
	clock   uint64
}

func newArtworkTiles(a *app) *artworkTiles {
	return &artworkTiles{app: a, post: a.update, cache: map[tileKey]*detailTile{}, jobs: map[tileKey]*tileJob{}, failed: map[tileKey]bool{}}
}

func (a *app) prepareTiles(id string, v *viewer) {
	if a.culture == nil || a.router.Path() != "/asset/"+id {
		return
	}
	if a.tiles == nil {
		a.tiles = newArtworkTiles(a)
	}
	if v.tiles == nil {
		v.tiles = &tileView{id: id}
	}
	a.tiles.activate(v.tiles)
}

func (s *artworkTiles) activate(v *tileView) {
	if s.active != v {
		s.deactivate()
		s.active = v
	}
	if v.pyramid != nil || v.loading || v.err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	v.ctx, v.cancel, v.loading = ctx, cancel, true
	go func() {
		p, err := s.app.culture.Tiles(ctx, v.id)
		s.post(func() {
			if v.ctx != ctx {
				return
			}
			v.loading, v.cancel, v.ctx = false, nil, nil
			if ctx.Err() == nil {
				v.pyramid, v.err = p, err
			}
			cancel()
		})
	}()
}

// Called on navigation as well as when the active artwork changes. Canceled
// work cannot publish pixels or errors into a later view of the artwork.
func (s *artworkTiles) deactivate() {
	if s.active != nil && s.active.cancel != nil {
		s.active.cancel()
		s.active.cancel, s.active.ctx, s.active.loading = nil, nil, false
	}
	for _, j := range s.jobs {
		j.cancel()
	}
	s.active, s.wanted, s.visible, s.pending = nil, nil, nil, nil
	s.failed = map[tileKey]bool{}
}

func (s *artworkTiles) clear() {
	s.deactivate()
	s.cache, s.bytes = map[tileKey]*detailTile{}, 0
}

// tileRange returns the tiles intersecting the viewport, with exclusive
// upper bounds. Exact tile edges must not request the next column or row.
func (p *culturePyramid) tileRange(z int, v *viewer) image.Rectangle {
	l := p.levels[z]
	x0, y0, x1, y1 := v.visible()
	if x1 <= x0 || y1 <= y0 {
		return image.Rectangle{}
	}
	return image.Rect(
		max(0, int(math.Floor(float64(x0)*float64(l.width)/float64(p.tileWidth)))),
		max(0, int(math.Floor(float64(y0)*float64(l.height)/float64(p.tileHeight)))),
		min(l.cols, int(math.Ceil(float64(x1)*float64(l.width)/float64(p.tileWidth)))),
		min(l.rows, int(math.Ceil(float64(y1)*float64(l.height)/float64(p.tileHeight)))),
	)
}

// Match the display's actual pixel density. Choose a coarser level if an
// unusually large viewport would otherwise exceed the decoded tile budget.
func (p *culturePyramid) plan(v *viewer, scale float32) []tileCoord {
	if v.fw <= 0 || v.fh <= 0 || v.sz <= 0 || len(p.levels) == 0 {
		return nil
	}
	want := v.fw * v.sz * max(scale, 1)
	z := len(p.levels) - 1
	for i, l := range p.levels {
		if float32(l.width) >= want {
			z = i
			break
		}
	}
	budget := min(128, tileCacheBytes/(p.tileWidth*p.tileHeight*6))
	r := p.tileRange(z, v)
	for z > 0 && r.Dx()*r.Dy() > budget {
		z--
		r = p.tileRange(z, v)
	}
	if r.Empty() || r.Dx()*r.Dy() > budget {
		return nil
	}
	coords := make([]tileCoord, 0, r.Dx()*r.Dy())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			coords = append(coords, tileCoord{z, x, y})
		}
	}
	// The center arrives first. A single surrounding ring prefetches nearby
	// detail only when it fits comfortably alongside the visible tiles.
	cx, cy := float64(r.Min.X+r.Max.X-1)/2, float64(r.Min.Y+r.Max.Y-1)/2
	distance := func(c tileCoord) float64 { return math.Pow(float64(c.x)-cx, 2) + math.Pow(float64(c.y)-cy, 2) }
	sort.Slice(coords, func(i, j int) bool { return distance(coords[i]) < distance(coords[j]) })
	l := p.levels[z]
	ring := r.Inset(-1).Intersect(image.Rect(0, 0, l.cols, l.rows))
	if ring.Dx()*ring.Dy() <= budget {
		for y := ring.Min.Y; y < ring.Max.Y; y++ {
			for x := ring.Min.X; x < ring.Max.X; x++ {
				if !image.Pt(x, y).In(r) {
					coords = append(coords, tileCoord{z, x, y})
				}
			}
		}
	}
	return coords
}

// request plans from where the view is headed rather than the frame being
// shown, so a zoom glide does not queue and cancel each level on its way.
func (s *artworkTiles) request(v *viewer, scale float32) {
	p := v.tiles.pyramid
	target := *v
	target.sz, target.sx, target.sy = v.zoom, v.tx, v.ty
	s.clock++
	s.wanted, s.visible, s.pending = map[tileKey]bool{}, map[tileKey]bool{}, nil
	var r image.Rectangle
	for i, c := range p.plan(&target, scale) {
		if i == 0 {
			r = p.tileRange(c.z, &target)
		}
		key := tileKey{p.base, c}
		s.wanted[key] = true
		if image.Pt(c.x, c.y).In(r) {
			s.visible[key] = true
		}
		if tile := s.cache[key]; tile != nil {
			tile.used = s.clock
		} else if s.jobs[key] == nil && !s.failed[key] {
			s.pending = append(s.pending, c)
		}
	}
	for key, j := range s.jobs {
		if !s.wanted[key] {
			j.cancel()
		}
	}
	for key := range s.failed {
		if !s.wanted[key] {
			delete(s.failed, key)
		}
	}
	s.pump()
}

func (s *artworkTiles) pump() {
	if s.active == nil || s.active.pyramid == nil {
		return
	}
	p := s.active.pyramid
	for len(s.jobs) < tileDownloads && len(s.pending) > 0 {
		c := s.pending[0]
		s.pending = s.pending[1:]
		key := tileKey{p.base, c}
		if !s.wanted[key] || s.cache[key] != nil || s.jobs[key] != nil || s.failed[key] {
			continue
		}
		ctx, cancel := context.WithCancel(context.Background())
		job := &tileJob{cancel: cancel}
		s.jobs[key] = job
		go func() {
			tile, err := s.load(ctx, p, c)
			s.post(func() {
				delete(s.jobs, key)
				if ctx.Err() == nil && s.active != nil && s.active.pyramid == p && s.wanted[key] {
					if err != nil {
						s.failed[key] = true
					} else {
						tile.used = s.clock
						s.cache[key] = tile
						s.bytes += tile.bytes
						s.trim()
					}
				}
				cancel()
				s.pump()
			})
		}()
	}
}

func (s *artworkTiles) load(ctx context.Context, p *culturePyramid, c tileCoord) (*detailTile, error) {
	var b []byte
	var err error
	for attempt := range 3 {
		b, err = s.app.culture.Tile(ctx, p, c)
		if err == nil || ctx.Err() != nil || errors.Is(err, errNotFound) {
			break
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	w, h := p.tileSize(c)
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	// Include space for the bitmap's smaller sampling levels in the budget.
	return &detailTile{bmp: ui.NewBitmap(dst), w: w, h: h, bytes: w * h * 6}, nil
}

func (s *artworkTiles) trim() {
	for s.bytes > tileCacheBytes || len(s.cache) > 128 {
		var oldest tileKey
		var found bool
		for key, tile := range s.cache {
			if !s.wanted[key] && (!found || tile.used < s.cache[oldest].used) {
				oldest, found = key, true
			}
		}
		if !found {
			return // plan bounds the wanted tiles to the cache budget
		}
		s.bytes -= s.cache[oldest].bytes
		delete(s.cache, oldest)
	}
}

// Draw cached lower levels first, then sharper tiles. Previously loaded
// detail remains visible across zoom steps while the new level downloads.
func (s *artworkTiles) draw(painter *ui.Painter, frame ui.Rect, v *viewer) {
	if v.tiles == nil || v.tiles.pyramid == nil || s.active != v.tiles {
		return
	}
	s.request(v, painter.Scale())
	p := v.tiles.pyramid
	keys := make([]tileKey, 0, len(s.cache))
	for key := range s.cache {
		if key.base == p.base && image.Pt(key.x, key.y).In(p.tileRange(key.z, v)) {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].z < keys[j].z })
	box := v.pictureRect(frame)
	scale := painter.Scale()
	snap := func(x float32) float32 { return float32(math.Round(float64(x*scale))) / scale }
	for _, key := range keys {
		tile := s.cache[key]
		tile.used = s.clock
		l := p.levels[key.z]
		x, y := float32(key.x*p.tileWidth), float32(key.y*p.tileHeight)
		x0, y0 := snap(box.X+x*box.W/float32(l.width)), snap(box.Y+y*box.H/float32(l.height))
		x1 := snap(box.X + (x+float32(tile.w))*box.W/float32(l.width))
		y1 := snap(box.Y + (y+float32(tile.h))*box.H/float32(l.height))
		painter.ImageRegion(tile.bmp, ui.Rect{}, ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, ui.ImageOptions{})
	}
}

func (v *viewer) pictureRect(frame ui.Rect) ui.Rect {
	w, h := v.fw*v.sz, v.fh*v.sz
	return ui.Rect{X: frame.X + v.w/2 + v.sx - w/2, Y: frame.Y + v.cy + v.sy - h/2, W: w, H: h}
}

// status reports on the tiles in view only: the prefetch ring neither keeps
// the spinner going nor raises an error. A pyramid that could not load is
// not a failure either; the viewer falls back to the regular large image.
func (s *artworkTiles) status(v *tileView) (loading, failed bool) {
	if s.active != v {
		return false, false
	}
	loading = v.loading
	for key := range s.visible {
		if s.failed[key] {
			failed = true
		} else if s.cache[key] == nil {
			loading = true
		}
	}
	return loading, failed
}

func (s *artworkTiles) retry(v *tileView) {
	s.failed = map[tileKey]bool{}
	v.err = nil
	s.activate(v)
}
