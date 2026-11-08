package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const cultureBase = "https://artsandculture.google.com"

type destinationKind string

const (
	artists destinationKind = "artists"
	museums destinationKind = "museums"
	places  destinationKind = "places"
)

func (kind destinationKind) categoryID() string {
	switch kind {
	case artists:
		return "artist"
	case places:
		return "place"
	}
	return ""
}

// cultureItem is an artist, a museum, a place, or a preview linking to its original
// artwork, story or virtual visit. Google IDs never enter the Met's feeds.
type cultureItem struct {
	Name, Subtitle, Image, Path     string
	Country, Address, Website, Maps string
}

func (i cultureItem) URL() string { return cultureBase + i.Path }

func (i cultureItem) route() string {
	if id := cultureAssetID(i.Path); id != "" {
		return "/asset/" + id
	}
	kind := places
	if strings.HasPrefix(i.Path, "/partner/") {
		kind = museums
	} else if u, err := url.Parse(i.Path); err == nil && u.Query().Get("categoryId") == "artist" {
		kind = artists
	}
	return "/" + string(kind) + "/view?" + url.Values{"path": {i.Path}}.Encode()
}

type cultureQuery struct {
	Kind         destinationKind
	Text, Prefix string
}

type culturePage struct {
	Items []cultureItem
	Total int
	Next  string
}

type destination struct {
	cultureItem
	Description                     string
	SourceName, SourceURL           string
	Total                           int
	Works, Stories, Visits, Museums []cultureItem
}

// Culture reads the public requests used by the Google Arts & Culture
// directories. This is an unofficial client, not a supported Google API.
// Keep its positional response format isolated here, and reject changed
// responses so that an error page never replaces a working cached copy.
type Culture struct {
	http        *http.Client
	cache       string
	api, images chan struct{}
	tiles       chan struct{} // deep-zoom tiles, so they never hold up previews
}

type tileRequestKey struct{}

// tileRequest marks a fetch as a deep-zoom tile, which waits for its own
// download slots instead of those of the pictures and thumbnails.
func tileRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, tileRequestKey{}, true)
}

func NewCulture(cacheDir string) *Culture {
	dir := filepath.Join(cacheDir, "artsandculture")
	_ = os.MkdirAll(dir, 0o755)
	return &Culture{http: &http.Client{Timeout: 25 * time.Second}, cache: dir, api: make(chan struct{}, 2), images: make(chan struct{}, 4), tiles: make(chan struct{}, tileDownloads)}
}

func (g *Culture) cachePath(u string) string {
	sum := sha256.Sum256([]byte(u))
	return filepath.Join(g.cache, hex.EncodeToString(sum[:]))
}

func (g *Culture) get(ctx context.Context, u string, age time.Duration, validate func([]byte) error) ([]byte, error) {
	path := g.cachePath(u)
	var stale []byte
	if info, err := os.Stat(path); err == nil {
		if b, err := os.ReadFile(path); err == nil && validate(b) == nil {
			if time.Since(info.ModTime()) < age {
				return b, nil
			}
			stale = b
		}
	}
	b, err := g.fetch(ctx, u)
	if err == nil {
		err = validate(b)
	}
	if err != nil {
		if stale != nil && ctx.Err() == nil && !errors.Is(err, errNotFound) {
			return stale, nil
		}
		return nil, err
	}
	// Concurrent image/page requests cannot leave a partially written copy.
	if f, err := os.CreateTemp(g.cache, ".download-*"); err == nil {
		name := f.Name()
		_, writeErr := f.Write(b)
		closeErr := f.Close()
		if writeErr == nil && closeErr == nil {
			_ = os.Rename(name, path)
		}
		_ = os.Remove(name)
	}
	return b, nil
}

func (g *Culture) fetch(ctx context.Context, u string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Metadata must not wait behind images from the page just left.
	sem := g.api
	if parsed, err := url.Parse(u); err == nil && cultureImageHost.MatchString(parsed.Hostname()) {
		sem = g.images
		if ctx.Value(tileRequestKey{}) != nil && g.tiles != nil {
			sem = g.tiles
		}
	}
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-sem }()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Atelier/0.1 (desktop art browser)")
	res, err := g.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Google Arts & Culture: %s", res.Status)
	}
	const maxBody = 16 << 20
	b, err := io.ReadAll(io.LimitReader(res.Body, maxBody+1))
	if len(b) > maxBody {
		return nil, errors.New("Google Arts & Culture response is too large")
	}
	return b, err
}

// request fetches a tagged record. check, when set, rejects records that
// decode but are wrong for this request, so they are never cached.
func (g *Culture) request(ctx context.Context, path string, params url.Values, tag string, age time.Duration, check func([]any) error) ([]any, error) {
	params.Set("hl", "en")
	params.Set("rt", "j")
	u := cultureBase + path + "?" + params.Encode()
	b, err := g.get(ctx, u, age, func(b []byte) error {
		r, err := cultureResponse(b, tag)
		if err == nil && check != nil {
			err = check(r)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return cultureResponse(b, tag)
}

// cultureResponse decodes Google's XSSI-prefixed JSON envelope as data,
// without evaluating any of the website's JavaScript.
func cultureResponse(b []byte, tag string) ([]any, error) {
	b = bytes.TrimSpace(b)
	if bytes.HasPrefix(b, []byte(")]}'")) {
		b = bytes.TrimSpace(b[4:])
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, errors.New("Google Arts & Culture returned an unreadable response")
	}
	if r := findCultureRecord(v, tag); validCultureRecord(r, tag) {
		return r, nil
	}
	return nil, errors.New("Google Arts & Culture returned an unexpected response")
}

func validCultureRecord(r []any, tag string) bool {
	if r == nil {
		return false
	}
	switch tag {
	case "stella.pr":
		_, count := at(r, 4).(float64)
		_, items := at(r, 2).([]any)
		return count && (items || at(r, 2) == nil)
	case "stella.sr":
		_, query := at(r, 1).(string)
		_, items := at(r, 2).([]any)
		// With no suggestions, Google omits the trailing items field.
		return query && (items || at(r, 2) == nil)
	case "stella.sp":
		_, query := at(r, 2).(string)
		_, topics := at(r, 8).([]any)
		return query && (topics || at(r, 8) == nil)
	case "stella.pp2":
		return textAt(arrayAt(r, 2), 1) != ""
	case "stella.ep":
		return (textAt(r, 3) == "place" || textAt(r, 3) == "artist") && textAt(r, 4) != ""
	case "stella.ap":
		v := arrayAt(r, 2)
		return textAt(v, 0) == "stella.av" && validCultureAssetID(textAt(v, 1)) && textAt(v, 2) != ""
	}
	return false
}

func findCultureRecord(v any, tag string) []any {
	a, ok := v.([]any)
	if !ok {
		return nil
	}
	if textAt(a, 0) == tag {
		return a
	}
	for _, child := range a {
		if r := findCultureRecord(child, tag); r != nil {
			return r
		}
	}
	return nil
}

func at(a []any, i int) any {
	if i < 0 || i >= len(a) {
		return nil
	}
	return a[i]
}
func arrayAt(a []any, i int) []any { v, _ := at(a, i).([]any); return v }
func textAt(a []any, i int) string { v, _ := at(a, i).(string); return v }
func numberAt(a []any, i int) int  { v, _ := at(a, i).(float64); return int(v) }

func cultureURL(raw string) string {
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return ""
	}
	return u.String()
}

// cultureImage requests a bounded image and retains the original's format
// options. Signed Street View/Maps URLs stay on the source website.
func cultureImage(raw string, width int) string {
	u, err := url.Parse(cultureURL(raw))
	if err != nil || u.Hostname() != "lh3.googleusercontent.com" {
		return ""
	}
	base, options, _ := strings.Cut(u.Path, "=")
	size := fmt.Sprint(width)
	u.Path = base + "=w" + size + "-h" + size
	for _, option := range strings.Split(options, "-") {
		if option == "" || cultureCropOption(option) {
			continue
		}
		u.Path += "-" + option
	}
	return u.String()
}

// cultureCropOption reports image options that set the size or crop it. With
// both w and h set, a crop flag would cut the picture to a square.
func cultureCropOption(option string) bool {
	switch option {
	case "c", "p", "n", "cc", "ci":
		return true
	}
	return len(option) > 1 && strings.ContainsRune("whs", rune(option[0])) && strings.Trim(option[1:], "0123456789") == ""
}

func cultureObject(a []any) (cultureItem, bool) {
	if textAt(a, 0) != "stella.common.cobject" && textAt(a, 0) != "gac.oi" {
		return cultureItem{}, false
	}
	path := textAt(a, 4)
	// Only relative links belonging to the source can become native routes.
	u, err := url.Parse(path)
	if err != nil || u.IsAbs() || u.Host != "" || !strings.HasPrefix(u.Path, "/") {
		return cultureItem{}, false
	}
	// Search results omit the category, which native routes need to
	// distinguish artists (including fashion designers) from places.
	if strings.HasPrefix(u.Path, "/entity/") && (numberAt(a, 5) == 3 || numberAt(a, 5) == 24) {
		q := u.Query()
		q.Set("categoryId", "artist")
		u.RawQuery = q.Encode()
	}
	item := cultureItem{Name: plainCultureText(textAt(a, 1)), Subtitle: plainCultureText(textAt(a, 2)), Image: cultureImage(textAt(a, 3), 900), Path: u.String()}
	if item.Name == "" {
		return cultureItem{}, false
	}
	if strings.HasPrefix(path, "/partner/") {
		item.Country = item.Subtitle
		// Some partners put their full description in the subtitle.
		item.Subtitle = ""
		if len(item.Country) < 100 {
			item.Subtitle = item.Country
		}
		location := arrayAt(a, 14)
		if textAt(location, 7) != "" {
			item.Subtitle = textAt(location, 7)
		}
		item.Address = textAt(location, 4)
		item.Website = cultureURL(textAt(location, 3))
		item.Maps = cultureURL(textAt(location, 2))
	}
	return item, true
}

func cultureObjects(v any, accept func([]any, cultureItem) bool) []cultureItem {
	var items []cultureItem
	seen := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		a, ok := v.([]any)
		if !ok {
			return
		}
		if item, ok := cultureObject(a); ok {
			if !seen[item.Path] && accept(a, item) {
				seen[item.Path] = true
				items = append(items, item)
			}
			return
		}
		for _, child := range a {
			walk(child)
		}
	}
	walk(v)
	return items
}

func (g *Culture) Directory(ctx context.Context, q cultureQuery, cursor string) (*culturePage, error) {
	p := url.Values{"pt": {cursor}, "s": {"36"}}
	path := "/api/objects/partner"
	tag := "stella.pr"
	if q.Kind == places || q.Kind == artists {
		path = "/api/objects/category"
		p.Set("categoryId", q.Kind.categoryID())
		p.Set("tab", "pop")
		if q.Prefix != "" {
			p.Set("tab", "az")
		}
		if q.Text != "" {
			path, tag = "/api/suggest", "stella.sr"
			p = url.Values{"q": {q.Text}}
			if q.Kind == artists {
				// Full search finds surnames, such as Monet, which the
				// autocomplete suggestions may omit. Only matched topics
				// belong here; the response also includes related artists.
				path, tag = "/api/search", "stella.sp"
			}
		}
	} else if q.Kind != museums {
		return nil, errors.New("unknown directory")
	}
	if q.Prefix != "" && q.Text == "" {
		p.Set("pr", q.Prefix)
	}
	if q.Text != "" {
		p.Set("q", q.Text)
	}
	r, err := g.request(ctx, path, p, tag, 24*time.Hour, nil)
	if err != nil {
		return nil, err
	}
	objects := at(r, 2)
	if tag == "stella.sp" {
		objects = at(r, 8)
	}
	items := cultureObjects(objects, func(a []any, item cultureItem) bool {
		if q.Kind == museums {
			return numberAt(a, 5) == 4 && strings.HasPrefix(item.Path, "/partner/")
		}
		if q.Kind == artists {
			return strings.HasPrefix(item.Path, "/entity/") &&
				(tag != "stella.sp" || numberAt(a, 5) == 3 || numberAt(a, 5) == 24)
		}
		return numberAt(a, 5) == 6 && strings.HasPrefix(item.Path, "/entity/")
	})
	page := &culturePage{Items: items, Total: numberAt(r, 4), Next: textAt(r, 8)}
	if tag == "stella.sr" || tag == "stella.sp" {
		page.Total, page.Next = len(items), ""
	}
	return page, nil
}

func (g *Culture) Destination(ctx context.Context, kind destinationKind, path string) (*destination, error) {
	u, err := url.Parse(path)
	if err != nil || u.IsAbs() || u.Host != "" {
		return nil, errors.New("invalid destination")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	d := &destination{cultureItem: cultureItem{Path: path}}
	if kind == museums && len(parts) == 2 && parts[0] == "partner" {
		r, err := g.request(ctx, "/api/partner", url.Values{"p": {parts[1]}}, "stella.pp2", 7*24*time.Hour, nil)
		if err != nil {
			return nil, err
		}
		info := arrayAt(r, 2)
		if textAt(info, 1) == "" {
			return nil, errors.New("museum details are unavailable")
		}
		d.Name, d.Subtitle = textAt(info, 1), textAt(info, 3)
		d.Description = plainCultureText(textAt(arrayAt(info, 2), 1))
		d.Image = cultureImage(textAt(info, 11), 1600)
		d.Website = cultureURL(textAt(arrayAt(info, 10), 1))
		visit := arrayAt(info, 14)
		d.Address = textAt(arrayAt(visit, 1), 2)
		d.Maps = cultureURL(textAt(visit, 3))
		d.Works = cultureObjects(at(info, 16), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/asset/") })
		d.Stories = cultureObjects(at(info, 7), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/story/") })
		d.Visits = cultureObjects(at(info, 8), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/streetview/") })
		return d, nil
	}
	category := kind.categoryID()
	if category == "" || (len(parts) != 2 && len(parts) != 3) || parts[0] != "entity" {
		return nil, errors.New("invalid destination")
	}
	entityID := parts[len(parts)-1]
	if len(entityID) < 2 || !validCultureAssetID(entityID) || !strings.ContainsRune("mgt", rune(entityID[0])) {
		return nil, errors.New("invalid entity ID")
	}
	entityID = "/" + entityID[:1] + "/" + entityID[1:]
	r, err := g.request(ctx, "/api/entity", url.Values{"entityId": {entityID}, "categoryId": {category}}, "stella.ep", 7*24*time.Hour, func(r []any) error {
		if textAt(r, 3) != category || textAt(r, 2) != entityID {
			return errors.New("Google Arts & Culture returned a different entity")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	d.Name, d.Image = textAt(r, 4), cultureImage(textAt(r, 5), 1600)
	if kind == artists {
		d.Subtitle = plainCultureText(textAt(r, 6))
	}
	d.Description = plainCultureText(textAt(arrayAt(r, 7), 1))
	d.SourceName, d.SourceURL = textAt(arrayAt(r, 8), 0), cultureURL(textAt(arrayAt(r, 8), 1))
	d.Total = numberAt(arrayAt(r, 9), 4)
	d.Works = cultureObjects(at(r, 9), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/asset/") })
	d.Museums = cultureObjects(at(r, 10), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/partner/") })
	d.Stories = cultureObjects(at(r, 11), func(_ []any, i cultureItem) bool { return strings.HasPrefix(i.Path, "/story/") })
	return d, nil
}

func plainCultureText(s string) string { return cultureHTMLText(s, " ") }

// cultureHTMLText reads the text of the source's HTML. br, p and div tags
// end a paragraph; paragraphs are joined with sep.
func cultureHTMLText(s, sep string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	var paragraphs []string
	var out strings.Builder
	flush := func() {
		if text := strings.Join(strings.Fields(out.String()), " "); text != "" {
			paragraphs = append(paragraphs, text)
		}
		out.Reset()
	}
	for {
		switch z.Next() {
		case html.ErrorToken:
			flush()
			return strings.Join(paragraphs, sep)
		case html.TextToken:
			out.Write(z.Text())
		case html.StartTagToken, html.EndTagToken, html.SelfClosingTagToken:
			switch name, _ := z.TagName(); string(name) {
			case "br", "p", "div":
				flush()
			}
		}
	}
}

func (g *Culture) Image(ctx context.Context, u string) ([]byte, error) {
	return g.get(ctx, u, 7*24*time.Hour, func(b []byte) error {
		if !strings.HasPrefix(http.DetectContentType(b), "image/") {
			return errors.New("invalid picture")
		}
		return nil
	})
}

func (g *Culture) Scan() cacheStats {
	var st cacheStats
	entries, _ := os.ReadDir(g.cache)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.Mode().IsRegular() {
			st.add(info.Size())
		}
	}
	return st
}

func (g *Culture) Clear() error {
	entries, err := os.ReadDir(g.cache)
	if err != nil {
		return err
	}
	var first error
	for _, e := range entries {
		if e.Type().IsRegular() {
			if err := os.Remove(filepath.Join(g.cache, e.Name())); err != nil && first == nil {
				first = err
			}
		}
	}
	return first
}
