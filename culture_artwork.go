package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"
)

// cultureArtwork keeps Google's IDs, credits and catalog fields separate
// from the Met's public-domain objects.
type cultureArtwork struct {
	ID, Title, Artist, Date, Image, Description, Path string
	Museum                                            cultureItem
	Details                                           []cultureProperty
}

type cultureProperty struct {
	Label, Value, URL string
}

func validCultureAssetID(id string) bool {
	if id == "" || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// cultureAssetID accepts source paths and pasted Google Arts & Culture
// links, including links without a title slug. It never accepts other hosts.
func cultureAssetID(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "artsandculture.google.com/") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return ""
	}
	if u.IsAbs() {
		if u.Scheme != "https" && u.Scheme != "http" || u.Host != "artsandculture.google.com" {
			return ""
		}
	} else if u.Host != "" {
		return ""
	}
	parts := strings.Split(strings.TrimSuffix(u.Path, "/"), "/")
	if (len(parts) != 3 && len(parts) != 4) || parts[0] != "" || parts[1] != "asset" {
		return ""
	}
	id := parts[len(parts)-1]
	if !validCultureAssetID(id) {
		return ""
	}
	return id
}

func (g *Culture) Artwork(ctx context.Context, id string) (*cultureArtwork, error) {
	if !validCultureAssetID(id) {
		return nil, errors.New("invalid artwork ID")
	}
	r, err := g.request(ctx, "/api/asset", url.Values{"assetId": {id}}, "stella.ap", 7*24*time.Hour, func(r []any) error {
		if textAt(arrayAt(r, 2), 1) != id {
			return errors.New("Google Arts & Culture returned a different artwork")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	v := arrayAt(r, 2)
	art := &cultureArtwork{
		ID: id, Title: plainCultureText(textAt(v, 2)), Date: plainCultureText(textAt(v, 3)),
		Artist:      plainCultureText(textAt(arrayAt(v, 6), 0)),
		Description: cultureDescription(textAt(arrayAt(v, 5), 1)),
		Path:        "/asset/" + id,
	}
	// The source's canonical link retains its human-readable title slug.
	if canonical := textAt(arrayAt(v, 15), 2); cultureAssetID(canonical) == id {
		u, _ := url.Parse(canonical)
		art.Path = u.EscapedPath()
	}
	art.Image = cultureURL(textAt(v, 8))
	if cultureImage(art.Image, 1) == "" {
		art.Image = cultureURL(textAt(v, 4))
	}
	if cultureImage(art.Image, 1) == "" {
		art.Image = ""
	}
	art.Museum, _ = cultureObject(arrayAt(v, 7))
	for _, value := range arrayAt(v, 12) {
		row, _ := value.([]any)
		label := plainCultureText(textAt(row, 0))
		for _, value := range arrayAt(row, 1) {
			entry, _ := value.([]any)
			text := plainCultureText(textAt(entry, 0))
			if text == "" {
				text = plainCultureText(textAt(arrayAt(entry, 2), 1))
			}
			if text != "" {
				art.Details = append(art.Details, cultureProperty{Label: label, Value: text, URL: cultureLink(textAt(entry, 1))})
			}
		}
	}
	return art, nil
}

// cultureLink accepts absolute links and the source's own site-relative ones.
func cultureLink(raw string) string {
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		raw = cultureBase + raw
	}
	return cultureURL(raw)
}

// cultureDescription keeps paragraph breaks for the artwork page; catalog
// labels and cards use the single-line plainCultureText.
func cultureDescription(raw string) string { return cultureHTMLText(raw, "\n\n") }
