// Atelier is a desktop app for exploring art, in native UI drawn by MyGo:
// featured stories and eras, a time machine,
// a zoomable viewer and favorites, over the public domain collection of
// The Metropolitan Museum of Art, with artists, museums and places from Google
// Arts & Culture.
//
//	go run .
package main

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// app is the state the window shows, read and written on the main thread.
type app struct {
	win     *mygo.Window
	met     *Met
	culture *Culture
	img     *images
	tiles   *artworkTiles
	router  *ui.Router

	feeds        map[string]*feed
	arts         map[int]*artState
	viewers      map[int]*viewer
	favs         *favorites
	layouts      map[string]*masonryState
	directories  map[destinationKind]*directoryState
	destinations map[string]*destinationState
	assets       map[string]*assetState

	// The home page's carousel.
	heroAt    int
	heroSince time.Time

	// The explore page's tab.
	exploreTab int

	// The time machine: the slider, and the year it settled on.
	timeSlider  float64
	timeYear    int
	timeTouched time.Time
	timeFeed    *feed

	// immersive shows an artwork alone, the window full screen.
	immersive bool

	settings settingsState
	lastPath string // the page the last frame showed
}

// menu is the menu bar: macOS's own, with Settings… in the app's menu.
func (a *app) menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Atelier", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout},
			mygo.Separator(),
			{Label: "Settings…", Accelerator: "CmdOrCtrl+,", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.update(func() { a.router.Push("/settings") })
			}},
			mygo.Separator(),
			{Role: mygo.RoleServices},
			mygo.Separator(),
			{Role: mygo.RoleHide},
			{Role: mygo.RoleHideOthers},
			{Role: mygo.RoleUnhide},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

// setImmersive shows the artwork alone, the window full screen, or ends it.
func (a *app) setImmersive(on bool) {
	if a.immersive == on {
		return
	}
	a.immersive = on
	if a.win != nil && a.win.IsFullScreen() != on {
		a.win.SetFullScreen(on)
	}
}

// update runs fn on the main thread and draws a new frame.
func (a *app) update(fn func()) {
	if a.win == nil {
		mygo.RunOnMain(fn)
		return
	}
	a.win.Update(fn)
}

// Colors of the app, warm paper and ink.
var (
	paper    = ui.Hex("#fbf9f5")
	card     = ui.Hex("#ffffff")
	ink      = ui.Hex("#1f1c18")
	inkSoft  = ui.Hex("#6c665d")
	inkFaint = ui.Hex("#a39c90")
	line     = ui.Hex("#e8e2d8")
	accent   = ui.Hex("#b4472b")
	wash     = ui.Hex("#f1ece3")
	night    = ui.Hex("#141311")
)

const serif = "Iowan Old Style, Georgia, Times New Roman"

func (a *app) theme(c *ui.Context) {
	t := *ui.LightTheme()
	t.Background = paper
	t.Surface = card
	t.SurfaceHover = wash
	t.Text = ink
	t.TextMuted = inkSoft
	t.Border = line
	t.Accent = accent
	t.AccentHover = ui.Hex("#9e3c23")
	t.AccentPressed = ui.Hex("#88331d")
	t.InactiveAccent = accent
	t.Radius = 8
	t.FontSize = 13
	c.SetTheme(&t)
}

func (a *app) view(c *ui.Context) {
	a.theme(c)
	ui.Column(c).Fill().AlignItems(ui.Stretch).Children(func() {
		if a.immersive && !strings.HasPrefix(a.router.Path(), "/artwork/") && !strings.HasPrefix(a.router.Path(), "/asset/") {
			a.setImmersive(false) // went back from the artwork
		}
		if !a.immersive {
			a.topBar(c)
		}
		if path := a.router.Path(); path != a.lastPath {
			if a.tiles != nil {
				a.tiles.deactivate()
			}
			// Destination details change in place, like the main sections.
			a.router.Transition = ui.TransitionAuto
			if path == "/artists/view" || path == "/museums/view" || path == "/places/view" ||
				a.lastPath == "/artists/view" || a.lastPath == "/museums/view" || a.lastPath == "/places/view" ||
				strings.HasPrefix(path, "/asset/") || strings.HasPrefix(a.lastPath, "/asset/") {
				a.router.Transition = ui.TransitionFade
			}
			// The settings page scans the cache each time it opens.
			if path == "/settings" {
				a.scanCache()
			}
			a.lastPath = path
		}
		a.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match("/home"):
				r.Title("Home")
				a.home(c)
			case r.Match("/artists") || r.Match("/explore"):
				r.Title("Artists")
				a.directory(c, artists)
			case r.Match("/museums"):
				r.Title("Museums")
				a.directory(c, museums)
			case r.Match("/places"):
				r.Title("Places")
				a.directory(c, places)
			case r.Match("/artists/view"):
				r.Title("Artist")
				a.destinationPage(c, artists, r.Query("path"))
			case r.Match("/museums/view"):
				r.Title("Museum")
				a.destinationPage(c, museums, r.Query("path"))
			case r.Match("/places/view"):
				r.Title("Place")
				a.destinationPage(c, places, r.Query("path"))
			case r.Match("/time"):
				r.Title("Time machine")
				a.timeMachine(c)
			case r.Match("/settings"):
				r.Title("Settings")
				a.settingsPage(c)
			case r.Match("/favorites"):
				r.Title("Favorites")
				a.favoritesPage(c)
			case r.Match("/collection"):
				title := r.Query("title")
				r.Title(title)
				a.collectionPage(c, title, r.Query("kicker"), queryFromRoute(r))
			case r.Match("/artwork/{id}"):
				id, _ := strconv.Atoi(r.Param("id"))
				r.Title("Artwork")
				a.artworkPage(c, id)
			case r.Match("/asset/{id}"):
				r.Title("Artwork")
				a.assetPage(c, r.Param("id"))
			default:
				ui.Text(c, "Not found").Padding(40)
			}
		}).Grow(1)
	})
}

// collectionPath is the page of a query's artworks.
func collectionPath(title, kicker string, q Query) string {
	v := url.Values{}
	v.Set("title", title)
	if kicker != "" {
		v.Set("kicker", kicker)
	}
	if q.Q != "" {
		v.Set("q", q.Q)
	}
	if q.Department > 0 {
		v.Set("dept", strconv.Itoa(q.Department))
	}
	if q.Highlight {
		v.Set("hl", "1")
	}
	if q.Artist {
		v.Set("artist", "1")
	}
	if q.HasDates {
		v.Set("from", strconv.Itoa(q.From))
		v.Set("to", strconv.Itoa(q.To))
	}
	if q.Medium != "" {
		v.Set("medium", q.Medium)
	}
	return "/collection?" + v.Encode()
}

func queryFromRoute(r *ui.Route) Query {
	q := Query{Q: r.Query("q"), Medium: r.Query("medium")}
	q.Department, _ = strconv.Atoi(r.Query("dept"))
	q.Highlight = r.Query("hl") == "1"
	q.Artist = r.Query("artist") == "1"
	if from := r.Query("from"); from != "" {
		q.HasDates = true
		q.From, _ = strconv.Atoi(from)
		q.To, _ = strconv.Atoi(r.Query("to"))
	}
	return q
}

func main() {
	cacheDir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		cacheDir = filepath.Join(os.TempDir(), "atelier")
	}
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		dataDir = cacheDir
	}
	a := &app{
		met:          NewMet(cacheDir),
		culture:      NewCulture(cacheDir),
		router:       ui.NewRouter("/home"),
		feeds:        map[string]*feed{},
		arts:         map[int]*artState{},
		viewers:      map[int]*viewer{},
		layouts:      map[string]*masonryState{},
		directories:  map[destinationKind]*directoryState{},
		destinations: map[string]*destinationState{},
		assets:       map[string]*assetState{},
		favs:         loadFavorites(dataDir),
		heroSince:    time.Now(),
		timeSlider:   1870,
		timeYear:     1870,
	}
	a.img = newImages(a)
	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:         "Atelier",
			Width:         1280,
			Height:        840,
			MinWidth:      860,
			MinHeight:     560,
			StateKey:      "main",
			TitleBarStyle: mygo.TitleBarHidden,
			// Centered down the top bar, which is topBarHeight high.
			TrafficLightPosition: &mygo.Point{X: 20, Y: (topBarHeight - 14) / 2},
			Content:              ui.View(a.view),
		})
		mygo.App.SetMenu(a.menu())
		// Leaving full screen otherwise (the green button, Esc, a
		// gesture) ends the immersive view too.
		a.win.OnLeaveFullScreen(func() {
			if a.immersive {
				a.immersive = false
				a.win.Invalidate()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
