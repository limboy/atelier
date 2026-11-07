package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type cultureTransport func(*http.Request) (*http.Response, error)

func (f cultureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func cultureFixture(fields map[int]any) []any {
	n := 0
	for i := range fields {
		n = max(n, i+1)
	}
	r := make([]any, n)
	for i, v := range fields {
		r[i] = v
	}
	return r
}

func cultureEnvelope(record any) string {
	b, _ := json.Marshal([]any{[]any{record, []any{"e", 2}}})
	return ")]}'\n\n" + string(b)
}

func cultureReply(body string) *http.Response {
	return &http.Response{StatusCode: 200, Status: "200 OK", Body: io.NopCloser(strings.NewReader(body))}
}

func sampleCultureObject(name, path string, kind int) []any {
	return []any{"stella.common.cobject", name, "", "//lh3.googleusercontent.com/ci/example", path, kind}
}

func TestCultureMuseumSearchAndPagination(t *testing.T) {
	museum := sampleCultureObject("Musée d’Orsay", "/partner/musee-dorsay-paris", 4)
	museum = append(museum, make([]any, 9)...)
	museum[2] = "France"
	museum[14] = []any{nil, nil, "https://maps.google.com/?q=Orsay", "https://museum.example/", "1 Museum Lane", 48.8, 2.3, "Paris, France"}
	body := cultureEnvelope([]any{"stella.pr", "PartnersQuery:test", []any{museum}, nil, 123, nil, nil, nil, "next+/="})
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/objects/partner" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("q") != "Paris & art" || q.Get("pt") != "previous+/=" || q.Get("hl") != "en" || q.Has("pr") {
			t.Errorf("incorrect search/pagination parameters: %v", q)
		}
		return cultureReply(body), nil
	})
	p, err := g.Directory(context.Background(), cultureQuery{Kind: museums, Text: "Paris & art", Prefix: "P"}, "previous+/=")
	if err != nil {
		t.Fatal(err)
	}
	if p.Total != 123 || p.Next != "next+/=" || len(p.Items) != 1 {
		t.Fatalf("page = %#v", p)
	}
	i := p.Items[0]
	if i.Name != "Musée d’Orsay" || i.Subtitle != "Paris, France" || i.Address != "1 Museum Lane" || i.Website != "https://museum.example/" {
		t.Fatalf("museum = %#v", i)
	}
	u, _ := url.Parse(i.route())
	if u.Path != "/museums/view" || u.Query().Get("path") != i.Path {
		t.Errorf("route = %s", i.route())
	}
}

func TestCulturePlaceSearchExcludesOtherEntityTypes(t *testing.T) {
	items := []any{
		sampleCultureObject("Tokyo National Museum", "/partner/tokyo-national-museum", 4),
		sampleCultureObject("Tokyo", "/entity/tokyo/m07dfk", 6),
		sampleCultureObject("Tokyo", "/entity/tokyo/m07dfk", 6),
		sampleCultureObject("Bombing of Tokyo", "/entity/bombing-of-tokyo/m0hx07", 5),
		sampleCultureObject("An artist", "/entity/an-artist/m123", 2),
		sampleCultureObject("Invalid", "//another.example/entity/test", 6),
	}
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/suggest" || r.URL.Query().Get("q") != "Tokyo" {
			t.Errorf("request = %s", r.URL)
		}
		return cultureReply(cultureEnvelope([]any{"stella.sr", "Tokyo", items})), nil
	})
	p, err := g.Directory(context.Background(), cultureQuery{Kind: places, Text: "Tokyo"}, "")
	if err != nil || len(p.Items) != 1 || p.Total != 1 || p.Next != "" {
		t.Fatalf("page = %#v, err = %v", p, err)
	}
	if !strings.HasPrefix(p.Items[0].route(), "/places/view?") {
		t.Fatal(p.Items[0].route())
	}
}

func TestCultureAlphabeticalPlacesUseServerCursor(t *testing.T) {
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if r.URL.Path != "/api/objects/category" || q.Get("categoryId") != "place" || q.Get("tab") != "az" || q.Get("pr") != "T" || !q.Has("pt") || q.Get("pt") != "cursor" {
			t.Errorf("incorrect alphabetical request: %s", r.URL)
		}
		return cultureReply(cultureEnvelope([]any{"stella.pr", "PrefixedAssets:test", []any{sampleCultureObject("Tokyo", "/entity/tokyo/m07dfk", 6)}, nil, 500, nil, nil, nil, "next"})), nil
	})
	p, err := g.Directory(context.Background(), cultureQuery{Kind: places, Prefix: "T"}, "cursor")
	if err != nil || p.Total != 500 || p.Next != "next" {
		t.Fatalf("page = %#v, err = %v", p, err)
	}
}

func TestCultureEmptySearchIsNotAnError(t *testing.T) {
	for _, tc := range []struct {
		kind   destinationKind
		record []any
	}{
		{places, []any{"stella.sr", "no-such-place"}},
		{museums, []any{"stella.pr", "PartnersQuery:empty", nil, nil, 0}},
	} {
		g := NewCulture(t.TempDir())
		g.http.Transport = cultureTransport(func(*http.Request) (*http.Response, error) { return cultureReply(cultureEnvelope(tc.record)), nil })
		p, err := g.Directory(context.Background(), cultureQuery{Kind: tc.kind, Text: "no-such-place"}, "")
		if err != nil || p.Total != 0 || len(p.Items) != 0 || p.Next != "" {
			t.Fatalf("%s: page=%#v, err=%v", tc.kind, p, err)
		}
	}
}

func TestCultureMuseumDetailsKeepNativeAndExternalContentApart(t *testing.T) {
	info := cultureFixture(map[int]any{
		1: "Example Museum", 2: []any{nil, "<p>Art &amp; history.</p><p>In Paris.</p>"}, 3: "Paris, France",
		7:  []any{[]any{sampleCultureObject("A story", "/story/story-id", 29)}},
		8:  []any{[]any{sampleCultureObject("Ground floor", "/streetview/tour-id", 20)}},
		10: []any{"Website", "https://museum.example/"}, 11: "//lh3.googleusercontent.com/ci/museum",
		14: []any{nil, []any{48.8, 2.3, "1 Museum Lane"}, nil, "https://maps.google.com/?q=Paris"},
		16: []any{[]any{sampleCultureObject("A painting", "/asset/painting-id", 8)}},
	})
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/partner" || r.URL.Query().Get("p") != "example-museum" {
			t.Errorf("request = %s", r.URL)
		}
		return cultureReply(cultureEnvelope([]any{"stella.ppw", nil, []any{"stella.pp2", "PartnerV2:test", info}})), nil
	})
	d, err := g.Destination(context.Background(), museums, "/partner/example-museum")
	if err != nil {
		t.Fatal(err)
	}
	if d.Name != "Example Museum" || d.Description != "Art & history. In Paris." || d.Address != "1 Museum Lane" || len(d.Works) != 1 || len(d.Stories) != 1 || len(d.Visits) != 1 || len(d.Museums) != 0 {
		t.Fatalf("details = %#v", d)
	}
}

func TestCulturePlaceDetailsUseKnowledgeGraphID(t *testing.T) {
	r := cultureFixture(map[int]any{
		0: "stella.ep", 1: "Entity:test", 2: "/g/121p5z1h", 3: "place", 4: "Denmark", 5: "//lh3.googleusercontent.com/ci/denmark",
		7: []any{nil, "A country in Europe.<br>Art &amp; culture."}, 8: []any{"Wikipedia", "https://en.wikipedia.org/wiki/Denmark"},
		9:  []any{"stella.pr", "EntityAssets:test", []any{sampleCultureObject("A painting", "/asset/painting-id", 8)}, nil, 4135},
		10: []any{"1 collection", []any{sampleCultureObject("A museum", "/partner/example", 4)}},
	})
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(req *http.Request) (*http.Response, error) {
		if req.URL.Query().Get("entityId") != "/g/121p5z1h" || req.URL.Query().Get("categoryId") != "place" {
			t.Errorf("request = %s", req.URL)
		}
		return cultureReply(cultureEnvelope(r)), nil
	})
	d, err := g.Destination(context.Background(), places, "/entity/denmark/g121p5z1h?categoryId=place")
	if err != nil || d.Total != 4135 || d.Description != "A country in Europe. Art & culture." || len(d.Museums) != 1 || d.SourceName != "Wikipedia" {
		t.Fatalf("details = %#v, err = %v", d, err)
	}
}

func TestCultureCachePreservesLastGoodResponse(t *testing.T) {
	for _, failure := range []string{"offline", "blocked", "changed schema", "incomplete response"} {
		t.Run(failure, func(t *testing.T) {
			g := NewCulture(t.TempDir())
			body := cultureEnvelope([]any{"stella.pr", "test", nil, nil, 0})
			calls := 0
			g.http.Transport = cultureTransport(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return cultureReply(body), nil
				}
				if failure == "offline" {
					return nil, errors.New("offline")
				}
				if failure == "blocked" {
					return cultureReply("<html>Blocked</html>"), nil
				}
				if failure == "incomplete response" {
					return cultureReply(cultureEnvelope([]any{"stella.pr", "test"})), nil
				}
				return cultureReply(cultureEnvelope([]any{"er", nil, 400})), nil
			})
			q := cultureQuery{Kind: museums}
			if _, err := g.Directory(context.Background(), q, ""); err != nil {
				t.Fatal(err)
			}
			if _, err := g.Directory(context.Background(), q, ""); err != nil || calls != 1 {
				t.Fatalf("fresh cache: calls=%d, err=%v", calls, err)
			}
			entries, _ := os.ReadDir(g.cache)
			path := g.cache + "/" + entries[0].Name()
			old := time.Now().Add(-48 * time.Hour)
			if err := os.Chtimes(path, old, old); err != nil {
				t.Fatal(err)
			}
			if _, err := g.Directory(context.Background(), q, ""); err != nil || calls != 2 {
				t.Fatalf("stale fallback: calls=%d, err=%v", calls, err)
			}
			if b, _ := os.ReadFile(path); string(b) != body {
				t.Fatal("invalid response overwrote the cache")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := g.Directory(ctx, q, ""); err == nil {
				t.Fatal("canceled request returned stale data")
			}
		})
	}
}

func TestCultureRejectsInvalidDestinationsBeforeRequest(t *testing.T) {
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(*http.Request) (*http.Response, error) { t.Fatal("invalid path made a request"); return nil, nil })
	for _, path := range []string{"https://example.com/partner/test", "//example.com/entity/m123", "/asset/not-a-place", "/entity/test/invalid", ""} {
		if _, err := g.Destination(context.Background(), places, path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
}

func TestCultureCacheCanBeCleared(t *testing.T) {
	g := NewCulture(t.TempDir())
	if err := os.WriteFile(g.cache+"/sample", []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if st := g.Scan(); st.Files != 1 || st.Bytes != 4 {
		t.Fatal(st)
	}
	if err := g.Clear(); err != nil {
		t.Fatal(err)
	}
	if st := g.Scan(); st.Files != 0 {
		t.Fatal(st)
	}
}

// Opt in when checking Google's live response format; the usual test run
// remains offline. This catches changes to the unofficial endpoints.
func TestCultureLive(t *testing.T) {
	if os.Getenv("ATELIER_LIVE_TEST") != "1" {
		t.Skip("set ATELIER_LIVE_TEST=1 to check Google Arts & Culture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	g := NewCulture(t.TempDir())
	for _, kind := range []destinationKind{museums, places} {
		q := cultureQuery{Kind: kind}
		first, err := g.Directory(ctx, q, "")
		if err != nil || len(first.Items) == 0 || first.Next == "" {
			t.Fatalf("%s first page: %#v, %v", kind, first, err)
		}
		next, err := g.Directory(ctx, q, first.Next)
		if err != nil || len(next.Items) == 0 || next.Items[0].Path == first.Items[0].Path {
			t.Fatalf("%s pagination: %#v, %v", kind, next, err)
		}
		alpha, err := g.Directory(ctx, cultureQuery{Kind: kind, Prefix: "T"}, "")
		if err != nil || len(alpha.Items) == 0 {
			t.Fatalf("%s A–Z: %#v, %v", kind, alpha, err)
		}
		search, err := g.Directory(ctx, cultureQuery{Kind: kind, Text: "Tokyo"}, "")
		if err != nil || len(search.Items) == 0 {
			t.Fatalf("%s search: %#v, %v", kind, search, err)
		}
		d, err := g.Destination(ctx, kind, search.Items[0].Path)
		if err != nil || d.Name == "" || d.Description == "" || len(d.Works) == 0 {
			t.Fatalf("%s details: %#v, %v", kind, d, err)
		}
		t.Logf("%s: %d entries; pagination, A–Z, search and %s details passed", kind, first.Total, d.Name)
	}
}
