package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
)

// feed is a query's artworks, loaded a page at a time. Read and written
// on the main thread.
type feed struct {
	q        Query
	pageSize int
	items    []*Artwork
	seen     map[int]bool
	total    int
	offset   int
	loading  bool
	done     bool
	err      error

	ctx    context.Context
	cancel context.CancelFunc
}

// feed returns the feed of q, loading its first page the first time.
func (a *app) feed(q Query, pageSize int) *feed {
	k := q.Key()
	if f, ok := a.feeds[k]; ok {
		return f
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &feed{q: q, pageSize: pageSize, seen: map[int]bool{}, ctx: ctx, cancel: cancel}
	a.feeds[k] = f
	a.more(f)
	return f
}

// dropFeed stops loading f and forgets it, for a feed the user moved on
// from before it loaded, so that its requests no longer hold up others.
func (a *app) dropFeed(f *feed) {
	if f == nil {
		return
	}
	f.cancel()
	if a.feeds[f.q.Key()] == f {
		delete(a.feeds, f.q.Key())
	}
}

// more loads the next page of f, unless it is loading or done. Works show
// one by one as they arrive.
func (a *app) more(f *feed) {
	if f.loading || f.done {
		return
	}
	f.loading = true
	offset, size := f.offset, f.pageSize
	go func() {
		ctx := f.ctx
		var total, read int
		var shown atomic.Int32
		var err error
		// Some objects have no picture we may show; read on until a page
		// has a few that do.
		for tries := 0; tries < 4 && int(shown.Load()) < size/2; tries++ {
			var ids []int
			ids, total, err = a.met.Search(ctx, f.q, offset+read, size)
			if err != nil || len(ids) == 0 {
				break
			}
			read += len(ids)
			t := total
			a.update(func() { f.total = t })
			var wg sync.WaitGroup
			for _, id := range ids {
				wg.Add(1)
				go func() {
					defer wg.Done()
					art, err := a.met.Artwork(ctx, id)
					if err != nil || art.Thumb() == "" {
						return
					}
					shown.Add(1)
					a.update(func() {
						if !f.seen[art.ID] {
							f.seen[art.ID] = true
							f.items = append(f.items, art)
						}
					})
				}()
			}
			wg.Wait()
			if offset+read >= total || ctx.Err() != nil {
				break
			}
		}
		if ctx.Err() != nil {
			return // dropped
		}
		a.update(func() {
			f.loading = false
			f.err = err
			f.total = total
			f.offset += read
			if err == nil && (read == 0 || f.offset >= total) {
				f.done = true
			}
		})
	}()
}

// artState is one artwork being loaded for its page.
type artState struct {
	art     *Artwork
	loading bool
	err     error
}

func (a *app) artwork(id int) *artState {
	if s, ok := a.arts[id]; ok {
		return s
	}
	s := &artState{loading: true}
	a.arts[id] = s
	go func() {
		art, err := a.met.Artwork(context.Background(), id)
		a.update(func() { s.art, s.err, s.loading = art, err, false })
	}()
	return s
}

// favorites are the artworks the user keeps, saved as their IDs, most
// recent first.
type favorites struct {
	path string
	ids  []int
}

func loadFavorites(dir string) *favorites {
	f := &favorites{path: filepath.Join(dir, "favorites.json")}
	if b, err := os.ReadFile(f.path); err == nil {
		_ = json.Unmarshal(b, &f.ids)
	}
	return f
}

func (f *favorites) has(id int) bool { return slices.Contains(f.ids, id) }

func (f *favorites) toggle(id int) {
	if i := slices.Index(f.ids, id); i >= 0 {
		f.ids = slices.Delete(f.ids, i, i+1)
	} else {
		f.ids = slices.Insert(f.ids, 0, id)
	}
	_ = os.MkdirAll(filepath.Dir(f.path), 0o755)
	if b, err := json.Marshal(f.ids); err == nil {
		_ = os.WriteFile(f.path, b, 0o644)
	}
}
