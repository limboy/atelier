package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"golang.org/x/image/draw"

	"github.com/egoist/mygo/ui"
)

// picture is an image downloaded and decoded, or on its way.
type picture struct {
	bmp     *ui.Bitmap
	w, h    int      // pixels of bmp
	tone    ui.Color // the picture's average color, for placeholders
	loading bool
	failed  bool
}

// Aspect is width over height, 0 while unknown.
func (p *picture) Aspect() float32 {
	if p == nil || p.h == 0 {
		return 0
	}
	return float32(p.w) / float32(p.h)
}

type picKey struct {
	url string
	max int
}

// images keeps the pictures the app showed. It is read and written on the
// main thread only; downloads run on goroutines and land through the
// window's Update.
type images struct {
	app *app
	m   map[picKey]*picture

	// Downloads wait in a stack: the newest asked for go first, as they
	// are what the page shown now needs, ahead of those of pages left.
	mu      sync.Mutex
	pending []func()
	wake    chan struct{}
}

func newImages(a *app) *images {
	s := &images{app: a, m: map[picKey]*picture{}, wake: make(chan struct{}, 1)}
	for range 6 {
		go s.worker()
	}
	return s
}

func (s *images) worker() {
	for {
		s.mu.Lock()
		if n := len(s.pending); n > 0 {
			job := s.pending[n-1]
			s.pending = s.pending[:n-1]
			s.mu.Unlock()
			job()
			continue
		}
		s.mu.Unlock()
		<-s.wake
	}
}

func (s *images) enqueue(job func()) {
	s.mu.Lock()
	s.pending = append(s.pending, job)
	s.mu.Unlock()
	for range 6 {
		select {
		case s.wake <- struct{}{}:
		default:
		}
	}
}

// get returns the picture at u scaled to fit maxDim pixels, starting its
// download the first time it is asked for.
func (s *images) get(u string, maxDim int) *picture {
	if u == "" {
		return nil
	}
	k := picKey{u, maxDim}
	if p, ok := s.m[k]; ok {
		return p
	}
	p := &picture{loading: true}
	s.m[k] = p
	s.enqueue(func() {
		bmp, w, h, tone, err := s.load(u, maxDim)
		s.app.update(func() {
			p.loading = false
			if err != nil {
				p.failed = true
				return
			}
			p.bmp, p.w, p.h, p.tone = bmp, w, h, tone
		})
	})
	return p
}

func (s *images) load(u string, maxDim int) (*ui.Bitmap, int, int, ui.Color, error) {
	data, err := s.app.met.Image(context.Background(), u)
	if err != nil {
		return nil, 0, 0, ui.Color{}, err
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, 0, 0, ui.Color{}, err
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if maxDim > 0 && (w > maxDim || h > maxDim) {
		if w >= h {
			h = max(1, h*maxDim/w)
			w = maxDim
		} else {
			w = max(1, w*maxDim/h)
			h = maxDim
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return ui.NewBitmap(dst), w, h, averageColor(dst), nil
}

// averageColor samples a grid of the picture's pixels.
func averageColor(m *image.RGBA) ui.Color {
	b := m.Bounds()
	var r, g, bl, n int
	for y := b.Min.Y; y < b.Max.Y; y += max(1, b.Dy()/24) {
		for x := b.Min.X; x < b.Max.X; x += max(1, b.Dx()/24) {
			c := m.RGBAAt(x, y)
			r += int(c.R)
			g += int(c.G)
			bl += int(c.B)
			n++
		}
	}
	if n == 0 {
		return ui.RGB(40, 40, 40)
	}
	c := color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255}
	return ui.RGB(c.R, c.G, c.B)
}
