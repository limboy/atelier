package main

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestCultureArtistDirectoryAndPagination(t *testing.T) {
	for _, prefix := range []string{"", "M"} {
		t.Run("prefix="+prefix, func(t *testing.T) {
			g := NewCulture(t.TempDir())
			g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
				q := r.URL.Query()
				tab := "pop"
				if prefix != "" {
					tab = "az"
				}
				if r.URL.Path != "/api/objects/category" || q.Get("categoryId") != "artist" || q.Get("tab") != tab || q.Get("pr") != prefix || q.Get("pt") != "previous+/=" {
					t.Errorf("incorrect artist directory request: %s", r.URL)
				}
				return cultureReply(cultureEnvelope([]any{"stella.pr", "artists", []any{
					sampleCultureObject("Claude Monet", "/entity/claude-monet/m01xnj?categoryId=artist", 3),
					sampleCultureObject("Madame Grès", "/entity/madame-gres/m0y5wzbs?categoryId=artist", 24),
				}, nil, 14380, nil, nil, nil, "next+/="})), nil
			})
			page, err := g.Directory(context.Background(), cultureQuery{Kind: artists, Prefix: prefix}, "previous+/=")
			if err != nil || page.Total != 14380 || page.Next != "next+/=" || len(page.Items) != 2 {
				t.Fatalf("artist directory = %#v, err = %v", page, err)
			}
			for _, item := range page.Items {
				u, _ := url.Parse(item.route())
				if u.Path != "/artists/view" || u.Query().Get("path") != item.Path {
					t.Errorf("artist route = %s", item.route())
				}
			}
		})
	}
}

func TestCultureArtistSearchOnlyIncludesMatchingArtists(t *testing.T) {
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/search" || r.URL.Query().Get("q") != "Monet" || r.URL.Query().Has("pr") {
			t.Errorf("incorrect artist search: %s", r.URL)
		}
		return cultureReply(cultureEnvelope(cultureFixture(map[int]any{
			0: "stella.sp", 2: "Monet",
			7: []any{sampleCultureObject("Hokusai", "/entity/hokusai/m0bwf4", 3)},
			8: []any{
				sampleCultureObject("Claude Monet", "/entity/claude-monet/m01xnj", 3),
				sampleCultureObject("Claude Monet", "/entity/claude-monet/m01xnj", 3),
				sampleCultureObject("Monet's Pond", "/entity/monets-pond/g11cn7fxtlb", 6),
				sampleCultureObject("Claude Monet", "/asset/claude-monet/painting-id", 8),
				sampleCultureObject("Monet museum", "/partner/monet", 4),
			},
		}))), nil
	})
	page, err := g.Directory(context.Background(), cultureQuery{Kind: artists, Text: "Monet", Prefix: "M"}, "")
	if err != nil || len(page.Items) != 1 || page.Total != 1 || page.Next != "" {
		t.Fatalf("artist search = %#v, err = %v", page, err)
	}
	u, _ := url.Parse(page.Items[0].route())
	if u.Path != "/artists/view" || page.Items[0].Path != "/entity/claude-monet/m01xnj?categoryId=artist" {
		t.Fatalf("search did not retain the artist category: %#v", page.Items[0])
	}
}

func TestCultureArtistDetailsSupportAllEntityIDs(t *testing.T) {
	for _, id := range []string{"m01xnj", "g123abc", "t218rckg2s5"} {
		t.Run(id, func(t *testing.T) {
			g := NewCulture(t.TempDir())
			g.http.Transport = cultureTransport(func(req *http.Request) (*http.Response, error) {
				entityID := "/" + id[:1] + "/" + id[1:]
				if req.URL.Path != "/api/entity" || req.URL.Query().Get("categoryId") != "artist" || req.URL.Query().Get("entityId") != entityID {
					t.Errorf("incorrect artist details request: %s", req.URL)
				}
				return cultureReply(cultureEnvelope(cultureFixture(map[int]any{
					0: "stella.ep", 2: entityID, 3: "artist", 4: "Example Artist", 6: "1840–1926",
					7: []any{nil, "<p>An artist &amp; painter.</p>"}, 8: []any{"Wikipedia", "https://en.wikipedia.org/wiki/Example"},
					9:  []any{"stella.pr", "works", []any{sampleCultureObject("Painting", "/asset/painting/painting-id", 8)}, nil, 293},
					11: []any{"Stories", []any{sampleCultureObject("A story", "/story/example", 29)}},
				}))), nil
			})
			d, err := g.Destination(context.Background(), artists, "/entity/example/"+id+"?categoryId=artist")
			if err != nil || d.Name != "Example Artist" || d.Subtitle != "1840–1926" || d.Description != "An artist & painter." || d.Total != 293 || len(d.Works) != 1 || len(d.Stories) != 1 || d.SourceName != "Wikipedia" {
				t.Fatalf("artist details = %#v, err = %v", d, err)
			}
		})
	}
}

func TestCultureArtistDetailsRejectWrongCategory(t *testing.T) {
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(*http.Request) (*http.Response, error) {
		return cultureReply(cultureEnvelope(cultureFixture(map[int]any{
			0: "stella.ep", 2: "/m/01xnj", 3: "place", 4: "Wrong category",
		}))), nil
	})
	if _, err := g.Destination(context.Background(), artists, "/entity/claude-monet/m01xnj?categoryId=artist"); err == nil {
		t.Fatal("an artist request accepted place details")
	}
	if st := g.Scan(); st.Files != 0 {
		t.Fatal("incorrect details were cached")
	}
}
