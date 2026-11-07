package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settingsState is the settings page's: what the cache holds, as last
// scanned, and what the user is doing with it.
type settingsState struct {
	report   *cacheReport
	scanning bool
	clearing bool
	err      string

	filter string
	list   ui.ListState

	// The question asked before clearing.
	confirm       bool
	confirmTitle  string
	confirmDetail string
	confirmKinds  []cacheKind
}

// scanCache reads what the cache holds, off the main thread.
func (a *app) scanCache() {
	s := &a.settings
	if s.scanning {
		return
	}
	s.scanning = true
	go func() {
		r := a.met.Scan()
		r.Culture = a.culture.Scan()
		a.update(func() { s.report, s.scanning = r, false })
	}()
}

// clearCache deletes the kinds of cached files given, forgets what the app
// keeps in memory of them, and scans again.
func (a *app) clearCache(kinds ...cacheKind) {
	s := &a.settings
	s.clearing, s.err = true, ""
	go func() {
		err := a.met.Clear(kinds...)
		for _, k := range kinds {
			if k == kindCulture {
				err = errors.Join(err, a.culture.Clear())
			}
		}
		a.update(func() {
			s.clearing = false
			if err != nil {
				s.err = err.Error()
			}
			for _, k := range kinds {
				switch k {
				case kindPicture:
					a.img.m = map[picKey]*picture{}
				case kindCulture:
					for _, s := range a.directories {
						if s.feed != nil {
							s.feed.cancel()
						}
					}
					a.directories = map[destinationKind]*directoryState{}
					a.destinations = map[string]*destinationState{}
					a.assets = map[string]*assetState{}
					for k := range a.img.m {
						if cultureImage(k.url, 1) != "" {
							delete(a.img.m, k)
						}
					}
				case kindArtwork, kindSearch:
					for _, f := range a.feeds {
						f.cancel()
					}
					a.feeds = map[string]*feed{}
					a.layouts = map[string]*masonryState{}
					a.timeFeed = nil
					if k == kindArtwork {
						a.arts = map[int]*artState{}
					}
				}
			}
			a.scanCache()
		})
	}()
}

// removeCached deletes one artwork's cached details and pictures.
func (a *app) removeCached(art *Artwork) {
	s := &a.settings
	go func() {
		err := a.met.RemoveArtwork(art)
		a.update(func() {
			if err != nil {
				s.err = err.Error()
			}
			delete(a.arts, art.ID)
			for k := range a.img.m {
				for _, u := range pictureURLs(art) {
					if k.url == u {
						delete(a.img.m, k)
					}
				}
			}
			a.scanCache()
		})
	}()
}

// askToClear asks before clearing kinds of the cache.
func (a *app) askToClear(title, detail string, kinds ...cacheKind) {
	s := &a.settings
	s.confirm, s.confirmTitle, s.confirmDetail, s.confirmKinds = true, title, detail, kinds
}

// The colors of the kinds of cached files, in the bar and its legend.
var (
	tonePictures = accent
	toneArtworks = ui.Hex("#5f7a5a")
	toneSearches = ui.Hex("#d9a547")
	toneCulture  = ui.Hex("#55788b")
)

func (a *app) settingsPage(c *ui.Context) {
	s := &a.settings
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		ui.Scroll(c.Key("storage")).Width(440).Shrink(0).BorderWidth(0, 1, 0, 0).BorderColor(line).Children(func() {
			ui.Column(c).Padding(36, 40, 40).Gap(26).Children(func() {
				pageHeader(c, "SETTINGS", "Storage", "What Atelier keeps on this Mac, so that pages open at once and work offline.")
				a.storageSummary(c)
			})
		})
		ui.Column(c.Key("artworks")).Grow(1).Padding(36, 40, 0).Gap(16).Children(func() {
			a.cachedArtworks(c)
		})
	})
	if ui.AlertDialog(c, &s.confirm, s.confirmTitle, s.confirmDetail, "Cancel", "Clear") == 1 {
		a.clearCache(s.confirmKinds...)
	}
}

func (a *app) storageSummary(c *ui.Context) {
	s := &a.settings
	r := s.report
	if r == nil {
		ui.Spinner(c).FontSize(18).AlignSelf(ui.Start)
		return
	}
	total := r.Total()
	ui.Column(c).Gap(18).Padding(22).Radius(12).Background(card).Border(1, line).Children(func() {
		ui.Row(c).AlignItems(ui.Baseline).Gap(10).Children(func() {
			ui.Text(c, humanBytes(total.Bytes)).Font(serif).FontSize(40).FontFeatures("tnum")
			ui.Text(c, "in "+plural(total.Files, "file", "files")).TextColor(inkSoft)
			ui.Spacer(c)
			if s.scanning || s.clearing {
				ui.Spinner(c).FontSize(14)
			}
		})
		// A bar of the kinds' shares of the total.
		ui.Row(c).Height(10).Radius(5).Clip().Background(wash).Children(func() {
			for i, part := range []struct {
				n    int64
				tone ui.Color
			}{{r.Pictures.Bytes, tonePictures}, {r.Artworks.Bytes, toneArtworks}, {r.Searches.Bytes, toneSearches}, {r.Culture.Bytes, toneCulture}} {
				if total.Bytes == 0 || part.n == 0 {
					continue
				}
				share := float32(part.n) / float32(total.Bytes) * 100
				seg := ui.Box(c.Key(i)).Height(10).Background(part.tone).Shrink(0)
				seg.WidthPercent(seg.AnimateTo("w", share, ui.Smooth))
			}
		})
		ui.Column(c).Gap(4).Children(func() {
			a.storageRow(c, "Pictures", "Thumbnails and full resolution scans. Kept until cleared.", tonePictures, r.Pictures, kindPicture)
			ui.Divider(c)
			a.storageRow(c, "Artwork details", "Titles, artists and dates. Refreshed after a week.", toneArtworks, r.Artworks, kindArtwork)
			ui.Divider(c)
			a.storageRow(c, "Search results", "The lists on each page. Refreshed after a day.", toneSearches, r.Searches, kindSearch)
			ui.Divider(c)
			a.storageRow(c, "Arts & Culture", "Museums, places, artwork details and images.", toneCulture, r.Culture, kindCulture)
		})
	})
	ui.Row(c).Gap(8).Wrap().Children(func() {
		b := ui.Button(c.Key("finder"), "").Padding(0, 14).Height(34).Radius(17)
		b.Children(func() {
			ui.Row(c).Gap(6).Children(func() {
				ui.Icon(c, icFolder).Size(15, 15)
				ui.Text(c, "Show in Finder")
			})
		})
		if b.Clicked() {
			// Both the Met and Google Arts & Culture have subfolders here.
			dir := filepath.Dir(r.Dir)
			_ = os.MkdirAll(dir, 0o755)
			mygo.Shell.OpenPath(dir)
		}
		clear := ui.Button(c.Key("clear-all"), "").Padding(0, 14).Height(34).Radius(17).Disabled(total.Files == 0 || s.clearing)
		clear.Children(func() {
			ui.Row(c).Gap(6).TextColor(ui.Hex("#b3261e")).Children(func() {
				ui.Icon(c, icTrash).Size(15, 15)
				ui.Text(c, "Clear all…")
			})
		})
		if clear.Clicked() {
			a.askToClear("Clear all cached content?",
				fmt.Sprintf("This deletes %s. Pages will download again as you open them, which needs a connection. Your favorites are kept.", humanBytes(total.Bytes)),
				kindPicture, kindArtwork, kindSearch, kindCulture)
		}
	})
	if s.err != "" {
		ui.Text(c, s.err).FontSize(12).TextColor(ui.Hex("#b3261e"))
	}
	ui.Text(c, "Favorites are saved apart from the cache, and clearing never touches them.").FontSize(12).TextColor(inkFaint)
}

func (a *app) storageRow(c *ui.Context, name, detail string, tone ui.Color, st cacheStats, kind cacheKind) {
	s := &a.settings
	ui.Row(c.Key(name)).Gap(12).Padding(10, 0).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(10, 10).Radius(5).Background(tone).Shrink(0)
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
				ui.Text(c, name).FontWeight(600)
				ui.Text(c, humanBytes(st.Bytes)+" · "+plural(st.Files, "file", "files")).FontSize(12).FontFeatures("tnum").TextColor(inkSoft)
			})
			ui.Text(c, detail).FontSize(12).TextColor(inkFaint)
		})
		b := ui.Button(c.Key("clear"), "Clear").Height(28).Radius(14).Disabled(st.Files == 0 || s.clearing)
		if b.Clicked() {
			a.askToClear("Clear "+strings.ToLower(name)+"?",
				fmt.Sprintf("This deletes %s of %s. They download again as you open pages.", humanBytes(st.Bytes), strings.ToLower(name)),
				kind)
		}
	})
}

func (a *app) cachedArtworks(c *ui.Context) {
	s := &a.settings
	r := s.report
	var items []cachedArtwork
	if r != nil {
		q := strings.ToLower(strings.TrimSpace(s.filter))
		for _, it := range r.Items {
			if q == "" || strings.Contains(strings.ToLower(it.Art.Title+" "+it.Art.Byline()), q) {
				items = append(items, it)
			}
		}
	}
	ui.Row(c).Gap(16).AlignItems(ui.End).Children(func() {
		ui.Column(c).Grow(1).Gap(4).Children(func() {
			ui.Text(c, "Cached artworks").Font(serif).FontSize(26)
			sub := "Artworks whose details are saved, with their pictures."
			if r != nil {
				sub = plural(len(r.Items), "artwork", "artworks") + " saved, with their pictures."
			}
			ui.Text(c, sub).TextColor(inkSoft)
		})
		ui.SearchField(c.Key("filter"), &s.filter).Placeholder("Filter").Label("Filter cached artworks").Width(220).Radius(16)
	})
	switch {
	case r == nil:
		ui.Box(c).Grow(1)
	case len(items) == 0:
		ui.Column(c).Grow(1).Center().Children(func() {
			msg := "Nothing cached yet."
			if len(r.Items) > 0 {
				msg = "No cached artwork matches."
			}
			ui.Text(c, msg).TextColor(inkSoft)
		})
	default:
		ui.List(c.Key("list"), &s.list, len(items), func(i int) {
			a.cachedRow(c.Key(items[i].Art.ID), items[i])
		}).Grow(1).Padding(0, 0, 24)
	}
}

func (a *app) cachedRow(c *ui.Context, it cachedArtwork) {
	art := it.Art
	ui.Row(c).Gap(14).Padding(8, 4).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(line).Children(func() {
		open := ui.ButtonBase(c.Key("open")).Grow(1).Gap(14).Justify(ui.Start).Radius(8).Padding(4).Cursor(ui.CursorPointer).
			Label("Open " + cleanTitle(art.Title))
		if open.Hovered() {
			open.Background(wash)
		}
		open.Children(func() {
			thumb := ui.Box(c).Size(48, 48).Radius(6).Clip().Background(wash).Shrink(0)
			thumb.Children(func() {
				// Only pictures still in the cache: the list must not
				// download what was just cleared.
				if it.Pictures > 0 && art.ImageSmall != "" && a.met.cached(art.ImageSmall) {
					if p := a.img.get(art.ImageSmall, 120); p != nil && p.bmp != nil {
						ui.Image(c, p.bmp).Fill().Fit(ui.Cover)
					}
				}
			})
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, cleanTitle(art.Title)).FontWeight(500).SingleLine()
				ui.Text(c, art.Byline()+dateSuffix(art.Date)).FontSize(12).SingleLine().TextColor(inkSoft)
			})
		})
		if open.Clicked() {
			a.router.Push("/artwork/" + strconv.Itoa(art.ID))
		}
		ui.Column(c).Width(120).AlignItems(ui.End).Gap(2).Shrink(0).Children(func() {
			ui.Text(c, humanBytes(it.Bytes)).FontFeatures("tnum")
			pics := "no pictures"
			if it.Pictures > 0 {
				pics = plural(it.Pictures, "picture", "pictures")
			}
			ui.Text(c, pics+" · "+it.Saved.Format("Jan 2")).FontSize(12).TextColor(inkFaint)
		})
		rm := ui.ButtonBase(c.Key("remove")).Size(32, 32).Radius(16).Shrink(0).Cursor(ui.CursorPointer).
			Label("Remove " + cleanTitle(art.Title) + " from the cache").Tooltip("Remove from cache").TextColor(inkFaint)
		if rm.Hovered() {
			rm.Background(wash).TextColor(ui.Hex("#b3261e"))
		}
		rm.Children(func() { ui.Icon(c, icTrash).Size(15, 15) }).OnClick(func() { a.removeCached(art) })
	})
}

// cached reports whether the cache holds a copy of u.
func (m *Met) cached(u string) bool {
	_, err := os.Stat(m.cachePath(u))
	return err == nil
}

// humanBytes writes n as people read sizes, as Finder does (1 KB is 1000
// bytes).
func humanBytes(n int64) string {
	switch {
	case n < 1000:
		return fmt.Sprintf("%d bytes", n)
	case n < 1000*1000:
		return fmt.Sprintf("%.0f KB", float64(n)/1000)
	case n < 1000*1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/1e9)
	}
}
