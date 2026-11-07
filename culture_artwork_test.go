package main

import (
	"bytes"
	"context"
	"image"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestCultureAssetLinks(t *testing.T) {
	for _, tc := range []struct{ link, id string }{
		{"https://artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw", "GgHsT2RumWxbtw"},
		{"https://artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw?hl=en#details", "GgHsT2RumWxbtw"},
		{"artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw", "GgHsT2RumWxbtw"},
		{"http://artsandculture.google.com/asset/GgHsT2RumWxbtw", "GgHsT2RumWxbtw"},
		{"ftp://artsandculture.google.com/asset/GgHsT2RumWxbtw", ""},
		{"/asset/GgHsT2RumWxbtw", "GgHsT2RumWxbtw"},
		{"/asset/%E5%86%A8%E5%B6%BD/abc_123-4", "abc_123-4"},
		{"https://example.com/asset/gleaners/GgHsT2RumWxbtw", ""},
		{"https://artsandculture.google.com.example.com/asset/GgHsT2RumWxbtw", ""},
		{"//example.com/asset/GgHsT2RumWxbtw", ""},
		{"https://someone@artsandculture.google.com/asset/GgHsT2RumWxbtw", ""},
		{"/asset/invalid%20id", ""},
		{"/partner/example-museum", ""},
		{"Gleaners", ""},
	} {
		if id := cultureAssetID(tc.link); id != tc.id {
			t.Errorf("%q: id=%q, want %q", tc.link, id, tc.id)
		}
	}
}

func TestCultureLinksAndText(t *testing.T) {
	for raw, want := range map[string]string{
		"/entity/jean-francois-millet/m0abc": cultureBase + "/entity/jean-francois-millet/m0abc",
		"https://museum.example/collection":  "https://museum.example/collection",
		"//museum.example/collection":        "https://museum.example/collection",
		"javascript:alert(1)":                "",
	} {
		if got := cultureLink(raw); got != want {
			t.Errorf("cultureLink(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := cultureDescription("a<p>b<br>c</p><div> d &amp; e </div>"); got != "a\n\nb\n\nc\n\nd & e" {
		t.Errorf("description = %q", got)
	}
	if got := plainCultureText("a<p>b<br>c</p><div> d &amp; e </div>"); got != "a b c d & e" {
		t.Errorf("plain text = %q", got)
	}
}

func TestCultureArtworkMetadata(t *testing.T) {
	id := "GgHsT2RumWxbtw"
	info := cultureFixture(map[int]any{
		0: "stella.av", 1: id, 2: "Gleaners", 3: "1857",
		4: "//lh3.googleusercontent.com/ci/preview", 8: "//lh3.googleusercontent.com/ci/artwork",
		5: []any{nil, "<p>Art &amp; history with <a href='/entity/test'>annotations</a>.</p><p>A second paragraph.</p>"},
		6: []any{"Jean-François Millet", nil},
		7: sampleCultureObject("Musée d’Orsay, Paris", "/partner/musee-dorsay-paris", 4),
		12: []any{
			[]any{"Rights", []any{[]any{"© Musée d'Orsay"}}, false},
			[]any{"Type", []any{[]any{"Oil on canvas"}}, false},
			[]any{"External Link", []any{[]any{"Museum collection", "https://museum.example/collection"}}, false},
			[]any{"Annotated field", []any{[]any{nil, nil, []any{nil, "<b>Value &amp; text</b>"}}}, false},
		},
		15: []any{"Gleaners", "", cultureBase + "/asset/gleaners/" + id},
	})
	g := NewCulture(t.TempDir())
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/asset" || r.URL.Query().Get("assetId") != id {
			t.Errorf("request = %s", r.URL)
		}
		return cultureReply(cultureEnvelope([]any{"stella.ap", "Asset:test", info})), nil
	})
	a, err := g.Artwork(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "Gleaners" || a.Artist != "Jean-François Millet" || a.Date != "1857" || a.Path != "/asset/gleaners/"+id {
		t.Fatalf("metadata = %#v", a)
	}
	if a.Description != "Art & history with annotations.\n\nA second paragraph." || a.Museum.Path != "/partner/musee-dorsay-paris" {
		t.Fatalf("description/museum = %#v", a)
	}
	if len(a.Details) != 4 || a.Details[0].Value != "© Musée d'Orsay" || a.Details[2].URL != "https://museum.example/collection" || a.Details[3].Value != "Value & text" {
		t.Fatalf("catalog = %#v", a.Details)
	}
}

func TestCultureArtworkRejectsIncompleteResponse(t *testing.T) {
	for _, info := range [][]any{{"stella.av", "requested"}, {"stella.av", "different-id", "A different artwork"}} {
		g := NewCulture(t.TempDir())
		g.http.Transport = cultureTransport(func(*http.Request) (*http.Response, error) {
			return cultureReply(cultureEnvelope([]any{"stella.ap", "Asset:test", info})), nil
		})
		if _, err := g.Artwork(context.Background(), "requested"); err == nil {
			t.Fatal("invalid artwork accepted")
		}
		if st := g.Scan(); st.Files != 0 {
			t.Fatal("invalid artwork was cached")
		}
	}
}

func TestCultureImageResizeIsBoundedAndRepeatable(t *testing.T) {
	u := cultureImage("//lh3.googleusercontent.com/ci/example=w900-h900-fautolight=1:autobacklight=1", 4096)
	if u != "https://lh3.googleusercontent.com/ci/example=w4096-h4096-fautolight=1:autobacklight=1" {
		t.Fatal(u)
	}
	if cultureImage(u, 4096) != u {
		t.Fatal("resizing added duplicate dimensions")
	}
	// Crop and size flags would square-crop a portrait inside the w/h box.
	u = cultureImage("https://lh3.googleusercontent.com/ci/example=s0-w600-h900-c-p-rw", 900)
	if u != "https://lh3.googleusercontent.com/ci/example=w900-h900-rw" {
		t.Fatal(u)
	}
}

func TestCultureArtworkLive(t *testing.T) {
	if os.Getenv("ATELIER_LIVE_TEST") != "1" {
		t.Skip("set ATELIER_LIVE_TEST=1 to check the live artwork")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := NewCulture(t.TempDir())
	a, err := g.Artwork(ctx, "GgHsT2RumWxbtw")
	if err != nil {
		t.Fatal(err)
	}
	if a.Title != "Gleaners" || a.Artist == "" || a.Description == "" || a.Museum.Path == "" || len(a.Details) == 0 {
		t.Fatalf("missing artwork data: %#v", a)
	}
	b, err := g.Image(ctx, cultureImage(a.Image, 4096))
	if err != nil {
		t.Fatal(err)
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || config.Width == 0 || config.Height == 0 {
		t.Fatalf("unreadable artwork image: %+v, %v", config, err)
	}
	credit := false
	for _, d := range a.Details {
		if strings.Contains(strings.ToLower(d.Label), "right") || strings.Contains(strings.ToLower(d.Label), "credit") {
			credit = true
		}
	}
	if !credit {
		t.Fatal("missing original credit/rights")
	}
	t.Logf("%s by %s: metadata, museum, credits and %d×%d image passed", a.Title, a.Artist, config.Width, config.Height)
}
