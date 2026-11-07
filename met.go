package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The Metropolitan Museum of Art's Open Access API: public domain works,
// free to use, no key. https://metmuseum.github.io/
const metBase = "https://collectionapi.metmuseum.org/public/collection/v1"

// Artwork is one object of the collection, as the API returns it.
type Artwork struct {
	ID                int      `json:"objectID"`
	Title             string   `json:"title"`
	Artist            string   `json:"artistDisplayName"`
	ArtistBio         string   `json:"artistDisplayBio"`
	ArtistNationality string   `json:"artistNationality"`
	Date              string   `json:"objectDate"`
	BeginDate         int      `json:"objectBeginDate"`
	EndDate           int      `json:"objectEndDate"`
	Medium            string   `json:"medium"`
	Dimensions        string   `json:"dimensions"`
	Department        string   `json:"department"`
	Culture           string   `json:"culture"`
	Period            string   `json:"period"`
	Dynasty           string   `json:"dynasty"`
	Classification    string   `json:"classification"`
	ObjectName        string   `json:"objectName"`
	CreditLine        string   `json:"creditLine"`
	Accession         string   `json:"accessionNumber"`
	City              string   `json:"city"`
	Country           string   `json:"country"`
	Region            string   `json:"region"`
	URL               string   `json:"objectURL"`
	Image             string   `json:"primaryImage"`
	ImageSmall        string   `json:"primaryImageSmall"`
	AdditionalImages  []string `json:"additionalImages"`
	PublicDomain      bool     `json:"isPublicDomain"`
	Highlight         bool     `json:"isHighlight"`
	Tags              []struct {
		Term string `json:"term"`
	} `json:"tags"`
}

// Thumb is the picture to show in grids.
func (a *Artwork) Thumb() string {
	if a.ImageSmall != "" {
		return a.ImageSmall
	}
	return a.Image
}

// Byline is the artist, or the culture, or the department.
func (a *Artwork) Byline() string {
	switch {
	case a.Artist != "":
		return a.Artist
	case a.Culture != "":
		return a.Culture
	default:
		return a.Department
	}
}

// Query is a search of the collection.
type Query struct {
	Q          string
	Department int
	Highlight  bool
	Artist     bool // match Q against artists and cultures only
	From, To   int  // years; both zero for any
	HasDates   bool
	Medium     string
}

func (q Query) values() url.Values {
	v := url.Values{}
	s := q.Q
	if s == "" {
		s = "*"
	}
	v.Set("q", s)
	v.Set("hasImages", "true")
	if q.Department > 0 {
		v.Set("departmentId", strconv.Itoa(q.Department))
	}
	if q.Highlight {
		v.Set("isHighlight", "true")
	}
	if q.Artist {
		v.Set("artistOrCulture", "true")
	}
	if q.HasDates {
		v.Set("dateBegin", strconv.Itoa(q.From))
		v.Set("dateEnd", strconv.Itoa(q.To))
	}
	if q.Medium != "" {
		v.Set("medium", q.Medium)
	}
	return v
}

// Key identifies the query, for caches.
func (q Query) Key() string { return q.values().Encode() }

// Met is the API's client: it caches responses on disk and in memory, and
// limits how many requests are in flight.
type Met struct {
	http  *http.Client
	cache string
	sem   chan struct{} // pictures in flight
	api   chan struct{} // API requests in flight
	tick  *time.Ticker  // paces requests to the API, whose firewall blocks bursts

	pause      sync.Mutex
	pauseUntil time.Time // the firewall turned us away: wait before asking again

	mu       sync.Mutex
	artworks map[int]*Artwork
	missing  map[int]bool
}

func NewMet(cacheDir string) *Met {
	dir := filepath.Join(cacheDir, "met")
	_ = os.MkdirAll(dir, 0o755)
	return &Met{
		http:     &http.Client{Timeout: 30 * time.Second},
		cache:    dir,
		sem:      make(chan struct{}, 4),
		api:      make(chan struct{}, 2),
		tick:     time.NewTicker(time.Second / 3),
		artworks: map[int]*Artwork{},
		missing:  map[int]bool{},
	}
}

// get fetches a URL, from the disk cache when it is younger than maxAge.
// An older copy is fetched again, but kept: when the fetch fails, as
// offline or while the API's firewall turns us away, get returns it
// rather than the error.
func (m *Met) get(ctx context.Context, u string, maxAge time.Duration) ([]byte, error) {
	path := m.cachePath(u)
	var stale []byte
	if st, err := os.Stat(path); err == nil {
		if b, err := os.ReadFile(path); err == nil {
			if maxAge <= 0 || time.Since(st.ModTime()) < maxAge {
				return b, nil
			}
			stale = b
		}
	}
	b, err := m.fetch(ctx, u, path, stale != nil)
	if err != nil && stale != nil && ctx.Err() == nil && !errors.Is(err, errNotFound) {
		return stale, nil
	}
	return b, err
}

// cachePath is where the copy of u is saved.
func (m *Met) cachePath(u string) string {
	sum := sha1.Sum([]byte(u))
	return filepath.Join(m.cache, hex.EncodeToString(sum[:]))
}

// fetch downloads u and saves it at path. With a copy to fall back on
// (hasStale), it gives up at the first failure, and at once while the API
// is paused, rather than retrying and waiting the pause out.
func (m *Met) fetch(ctx context.Context, u, path string, hasStale bool) ([]byte, error) {
	api := strings.HasPrefix(u, metBase)
	if api && hasStale && m.paused() {
		return nil, errPaused
	}
	sem := m.sem
	if api {
		sem = m.api
	}
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-sem }()

	attempts := 3
	if hasStale {
		attempts = 1
	}
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		if api {
			m.pause.Lock()
			wait := time.Until(m.pauseUntil)
			m.pause.Unlock()
			if wait > 0 {
				time.Sleep(wait)
			}
			select {
			case <-m.tick.C:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Atelier/0.1 (desktop art browser)")
		res, err := m.http.Do(req)
		if err != nil {
			last = err
			m.backoff(attempt, attempts, 400*time.Millisecond)
			continue
		}
		b, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			last = err
			continue
		}
		switch {
		case res.StatusCode == http.StatusNotFound:
			return nil, errNotFound
		case res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusForbidden:
			// Rate limited: every request backs off, not just this one.
			last = fmt.Errorf("%s: %s", u, res.Status)
			m.pause.Lock()
			if until := time.Now().Add(time.Duration(attempt+1) * 20 * time.Second); until.After(m.pauseUntil) {
				m.pauseUntil = until
			}
			m.pause.Unlock()
			continue
		case res.StatusCode >= 500:
			last = fmt.Errorf("%s: %s", u, res.Status)
			m.backoff(attempt, attempts, 800*time.Millisecond)
			continue
		case res.StatusCode != http.StatusOK:
			return nil, fmt.Errorf("%s: %s", u, res.Status)
		}
		if api && (len(b) == 0 || b[0] != '{') {
			// A firewall's page in place of JSON: never cache it.
			last = fmt.Errorf("%s: unexpected response", u)
			m.backoff(attempt, attempts, 800*time.Millisecond)
			continue
		}
		_ = os.WriteFile(path, b, 0o644)
		return b, nil
	}
	return nil, last
}

var (
	errNotFound = errors.New("not found")
	errPaused   = errors.New("the API is paused after turning us away")
)

// backoff waits before the next attempt, longer each time, and not at all
// after the last one.
func (m *Met) backoff(attempt, attempts int, step time.Duration) {
	if attempt+1 < attempts {
		time.Sleep(time.Duration(attempt+1) * step)
	}
}

// paused reports whether the API turned us away a moment ago.
func (m *Met) paused() bool {
	m.pause.Lock()
	defer m.pause.Unlock()
	return time.Now().Before(m.pauseUntil)
}

// Search returns a page of the IDs of the objects matching q, and how
// many match.
func (m *Met) Search(ctx context.Context, q Query, offset, limit int) ([]int, int, error) {
	v := q.values()
	v.Set("offset", strconv.Itoa(offset))
	v.Set("limit", strconv.Itoa(limit))
	b, err := m.get(ctx, metBase+".1/search?"+v.Encode(), 24*time.Hour)
	if err != nil {
		return nil, 0, err
	}
	var r struct {
		Total int   `json:"total"`
		IDs   []int `json:"objectIDs"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, 0, err
	}
	return r.IDs, r.Total, nil
}

// Artwork returns an object, from the cache when it has it.
func (m *Met) Artwork(ctx context.Context, id int) (*Artwork, error) {
	m.mu.Lock()
	a, ok := m.artworks[id]
	gone := m.missing[id]
	m.mu.Unlock()
	if ok {
		return a, nil
	}
	if gone {
		return nil, errNotFound
	}
	b, err := m.get(ctx, fmt.Sprintf("%s/objects/%d", metBase, id), 7*24*time.Hour)
	if err != nil {
		if errors.Is(err, errNotFound) {
			m.mu.Lock()
			m.missing[id] = true
			m.mu.Unlock()
		}
		return nil, err
	}
	a = &Artwork{}
	if err := json.Unmarshal(b, a); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.artworks[id] = a
	m.mu.Unlock()
	return a, nil
}

// Artworks fetches objects in parallel, keeping their order and only those
// with a picture.
func (m *Met) Artworks(ctx context.Context, ids []int) []*Artwork {
	out := make([]*Artwork, len(ids))
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if a, err := m.Artwork(ctx, id); err == nil && a.Thumb() != "" {
				out[i] = a
			}
		}()
	}
	wg.Wait()
	kept := out[:0]
	for _, a := range out {
		if a != nil {
			kept = append(kept, a)
		}
	}
	return kept
}

// Image fetches the bytes of a picture, cached on disk for good.
func (m *Met) Image(ctx context.Context, u string) ([]byte, error) {
	return m.get(ctx, u, 0)
}

// cleanTitle shortens the long titles some objects have.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "Untitled"
	}
	return s
}
