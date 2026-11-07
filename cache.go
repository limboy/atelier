package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// cacheKind is what a file of the cache holds. Files are named by the hash
// of their URL, so their content tells them apart: the API answers JSON,
// and everything else is a picture.
type cacheKind int

const (
	kindPicture cacheKind = iota
	kindArtwork
	kindSearch
	kindCulture
)

// cacheStats counts the files of one kind and their bytes.
type cacheStats struct {
	Files int
	Bytes int64
}

func (s *cacheStats) add(n int64) { s.Files++; s.Bytes += n }

// cachedArtwork is an artwork whose details are cached, with the bytes of
// those details and of its pictures in the cache.
type cachedArtwork struct {
	Art      *Artwork
	Bytes    int64
	Pictures int
	Saved    time.Time
}

// cacheReport is what the cache holds.
type cacheReport struct {
	Dir                          string
	Pictures, Artworks, Searches cacheStats
	Culture                      cacheStats
	Items                        []cachedArtwork // most recently saved first
}

func (r *cacheReport) Total() cacheStats {
	return cacheStats{
		Files: r.Pictures.Files + r.Artworks.Files + r.Searches.Files + r.Culture.Files,
		Bytes: r.Pictures.Bytes + r.Artworks.Bytes + r.Searches.Bytes + r.Culture.Bytes,
	}
}

// classify reads enough of a cached file to tell its kind, and for an
// artwork, the artwork.
func classify(path string) (cacheKind, *Artwork) {
	f, err := os.Open(path)
	if err != nil {
		return kindPicture, nil
	}
	defer f.Close()
	var first [1]byte
	if _, err := io.ReadFull(f, first[:]); err != nil || first[0] != '{' {
		return kindPicture, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return kindSearch, nil
	}
	var a Artwork
	if json.Unmarshal(b, &a) == nil && a.ID > 0 {
		return kindArtwork, &a
	}
	return kindSearch, nil
}

// pictureURLs are the URLs an artwork's pictures may be cached under.
func pictureURLs(a *Artwork) []string {
	urls := []string{a.ImageSmall, a.Image, webLarge(a.Image)}
	for _, u := range a.AdditionalImages {
		urls = append(urls, u, webLarge(u))
	}
	seen := map[string]bool{"": true}
	out := urls[:0]
	for _, u := range urls {
		if !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

// Scan reports what the cache holds. It reads the cache's files, so call
// it off the main thread.
func (m *Met) Scan() *cacheReport {
	r := &cacheReport{Dir: m.cache}
	entries, _ := os.ReadDir(m.cache)
	pictures := map[string]int64{} // file name → bytes
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		path := filepath.Join(m.cache, e.Name())
		kind, art := classify(path)
		switch kind {
		case kindPicture:
			r.Pictures.add(info.Size())
			pictures[e.Name()] = info.Size()
		case kindSearch:
			r.Searches.add(info.Size())
		case kindArtwork:
			r.Artworks.add(info.Size())
			r.Items = append(r.Items, cachedArtwork{Art: art, Bytes: info.Size(), Saved: info.ModTime()})
		}
	}
	for i := range r.Items {
		it := &r.Items[i]
		for _, u := range pictureURLs(it.Art) {
			if n, ok := pictures[filepath.Base(m.cachePath(u))]; ok {
				it.Bytes += n
				it.Pictures++
			}
		}
	}
	sort.Slice(r.Items, func(i, j int) bool { return r.Items[i].Saved.After(r.Items[j].Saved) })
	return r
}

// Clear deletes the cached files of the kinds given, and forgets what it
// keeps in memory of them.
func (m *Met) Clear(kinds ...cacheKind) error {
	want := map[cacheKind]bool{}
	for _, k := range kinds {
		want[k] = true
	}
	entries, err := os.ReadDir(m.cache)
	if err != nil {
		return err
	}
	var first error
	for _, e := range entries {
		path := filepath.Join(m.cache, e.Name())
		if kind, _ := classify(path); want[kind] {
			if err := os.Remove(path); err != nil && first == nil {
				first = err
			}
		}
	}
	if want[kindArtwork] {
		m.forget()
	}
	return first
}

// RemoveArtwork deletes an artwork's cached details and pictures.
func (m *Met) RemoveArtwork(a *Artwork) error {
	var first error
	for _, u := range append(pictureURLs(a), fmtObjectURL(a.ID)) {
		if err := os.Remove(m.cachePath(u)); err != nil && !os.IsNotExist(err) && first == nil {
			first = err
		}
	}
	m.mu.Lock()
	delete(m.artworks, a.ID)
	m.mu.Unlock()
	return first
}

// forget drops the artworks kept in memory.
func (m *Met) forget() {
	m.mu.Lock()
	m.artworks = map[int]*Artwork{}
	m.missing = map[int]bool{}
	m.mu.Unlock()
}
