package main

import (
	"os"
	"testing"
)

// fillCache saves an artwork with two of its pictures, a picture of
// nothing cached, and a search.
func fillCache(t *testing.T, m *Met) *Artwork {
	t.Helper()
	art := &Artwork{ID: 7, Title: "Whalers", ImageSmall: "https://img/web-large/a.jpg", Image: "https://img/original/a.jpg"}
	files := map[string]string{
		fmtObjectURL(7):                `{"objectID":7,"title":"Whalers","primaryImageSmall":"https://img/web-large/a.jpg","primaryImage":"https://img/original/a.jpg"}`,
		art.ImageSmall:                 "JPEG-small",
		art.Image:                      "JPEG-original-bigger",
		"https://img/other.jpg":        "JPEG",
		metBase + ".1/search?q=whales": `{"total":1,"objectIDs":[7]}`,
	}
	for u, body := range files {
		if err := os.WriteFile(m.cachePath(u), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return art
}

func TestScan(t *testing.T) {
	m := NewMet(t.TempDir())
	fillCache(t, m)
	r := m.Scan()
	if r.Pictures.Files != 3 || r.Artworks.Files != 1 || r.Searches.Files != 1 {
		t.Fatalf("pictures %d, artworks %d, searches %d; want 3, 1, 1", r.Pictures.Files, r.Artworks.Files, r.Searches.Files)
	}
	if r.Pictures.Bytes != int64(len("JPEG-small")+len("JPEG-original-bigger")+len("JPEG")) {
		t.Errorf("pictures take %d bytes", r.Pictures.Bytes)
	}
	if len(r.Items) != 1 {
		t.Fatalf("%d cached artworks, want 1", len(r.Items))
	}
	it := r.Items[0]
	if it.Art.Title != "Whalers" || it.Pictures != 2 {
		t.Errorf("item %q with %d pictures, want Whalers with 2", it.Art.Title, it.Pictures)
	}
	if total := r.Total(); total.Files != 5 {
		t.Errorf("%d files in all, want 5", total.Files)
	}
}

func TestClearKeepsOtherKinds(t *testing.T) {
	m := NewMet(t.TempDir())
	fillCache(t, m)
	if err := m.Clear(kindPicture); err != nil {
		t.Fatal(err)
	}
	r := m.Scan()
	if r.Pictures.Files != 0 || r.Artworks.Files != 1 || r.Searches.Files != 1 {
		t.Fatalf("after clearing pictures: pictures %d, artworks %d, searches %d", r.Pictures.Files, r.Artworks.Files, r.Searches.Files)
	}
	if err := m.Clear(kindArtwork, kindSearch); err != nil {
		t.Fatal(err)
	}
	if total := m.Scan().Total(); total.Files != 0 {
		t.Errorf("%d files left after clearing everything", total.Files)
	}
}

func TestRemoveArtwork(t *testing.T) {
	m := NewMet(t.TempDir())
	art := fillCache(t, m)
	m.artworks[art.ID] = art
	if err := m.RemoveArtwork(art); err != nil {
		t.Fatal(err)
	}
	r := m.Scan()
	if len(r.Items) != 0 || r.Pictures.Files != 1 || r.Searches.Files != 1 {
		t.Fatalf("after removing: %d artworks, %d pictures, %d searches; want 0, 1 (another's), 1", len(r.Items), r.Pictures.Files, r.Searches.Files)
	}
	if _, ok := m.artworks[art.ID]; ok {
		t.Error("the artwork is still kept in memory")
	}
}
