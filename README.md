# Atelier

A desktop app for exploring art, built with [mygo](https://github.com/egoist/mygo)'s native UI: no webview, drawn on the GPU from Go. It browses the public domain collection of [The Metropolitan Museum of Art](https://www.metmuseum.org/) through its [Open Access API](https://metmuseum.github.io/), and artists, museums and places from [Google Arts & Culture](https://artsandculture.google.com/).

- **Home**: a featured carousel, an art of the day, and rows of themed stories (Impressionism, Van Gogh, ukiyo-e, Ancient Egypt, armor…) that scroll sideways.
- **Artists**: the [Google Arts & Culture artist directory](https://artsandculture.google.com/category/artist), with featured artists, A–Z browsing and search. Native artist pages show dates, biographies, selected works and stories; artworks open in the shared viewer.
- **Museums**: the [Google Arts & Culture partner directory](https://artsandculture.google.com/partner), with featured collections, A–Z browsing, search by museum or city, and pages of results. Native detail pages show descriptions, collection highlights, stories, websites, addresses and map links.
- **Places**: [countries, cities and landmarks](https://artsandculture.google.com/category/place), with featured and A–Z browsing, place name suggestions, collection previews and related museums. Museums and artworks open inside Atelier; stories and virtual visits open on Google Arts & Culture in your browser.
- **Time machine**: a slider and eras from 3000 BCE to 1950.
- **Artwork**: a viewer that zooms with the trackpad, the wheel, a double click or ⌘= ⌘− ⌘0, pans by dragging, and loads the full resolution scan as you zoom in. **F** shows the artwork alone, full screen; **Esc** leaves full screen, and otherwise goes back to where you came from. Beside it, the details, its themes, more by the artist and from its department.
- **Google Arts & Culture artworks**: collection highlights open in the native viewer, with zoom, pan, full screen, the full description, original catalog fields and credits, and a link to the owning museum. Paste an artwork URL, such as `https://artsandculture.google.com/asset/gleaners/GgHsT2RumWxbtw`, into the top search field and press Return to open it directly. Back returns to the museum or place you came from, with an in-place fade.
- **Search** (⌘F) and **Favorites**.
- **Settings** (⌘, or the gear): what the cache holds, as pictures, artwork details, search results, and museums and places, with their sizes; the cached Met artworks, filtered and removed one by one; clearing each kind or all, and the folder in Finder.

## Developing

Go 1.27.3 or later and [Bun](https://bun.sh).

```sh
bun install
bun run dev       # develop: rebuilds and restarts the app as files change
bun run build     # build/darwin-arm64/Atelier.app and a .dmg
bun run build:dmg # verified universal DMG for Apple Silicon and Intel (macOS only)
go test .         # offline: API clients and disk caches through fake transports
ATELIER_LIVE_TEST=1 go test -run 'TestCulture.*Live' -v . # optional: live directories, pagination, search, details and artwork image
```

To install the build you made:

```sh
bun run build && ditto build/darwin-arm64/Atelier.app /Applications/Atelier.app
```

`mygo.config.ts` reads Apple signing and notarization settings from `APPLE_SIGNING_IDENTITY`, `APPLE_NOTARY_PROFILE` and `APPLE_KEYCHAIN_PATH`; without them, builds are signed ad hoc for this Mac.

## Code

- `main.go`: the app's state, theme, routes and window.
- `met.go`: the client of the Met's API, with its disk cache and pacing.
- `culture.go`: the Google Arts & Culture directory and detail client, response parsing, pagination and separate disk cache.
- `culture_artwork.go`: Google Arts & Culture artwork metadata, catalog fields, descriptions and source links.
- `asset.go`: native Google Arts & Culture artwork pages, using the shared viewer in `artwork.go`.
- `destinations.go`: the native artist, museum and place directories, search, detail pages and home shortcuts.
- `state.go`: feeds of a search's artworks, loaded a page at a time, and favorites.
- `images.go`: pictures downloaded, scaled and decoded off the main thread, newest first.
- `cache.go`: what the cache holds, by kind, and clearing it.
- `components.go`: the top bar, artwork cards, rows and the masonry grid.
- `home.go`, `collection.go` (collections, favorites, the time machine), `artwork.go` (the viewer and details), `settings.go` (storage): the pages.
- `resources/icon.png`: the app's icon, rendered from `design/icon.svg` with `rsvg-convert -w 1024 -h 1024 design/icon.svg -o resources/icon.png`.

## Data and the API

The API's firewall turns away bursts of requests with a 403. The app paces its requests to about three a second, two at a time, and every request backs off for 20 seconds or more after a 403; loads you move away from, as years passed on the time machine's slider, are cancelled.

Responses are cached in `~/Library/Caches/Atelier/met`: pictures for good, artworks for a week and searches for a day. An expired copy is fetched again, and shown when that fails, offline or while the API turns the app away. The cache has no size limit yet; Settings shows and clears it.

Artists, museums and places use the public JSON requests made by the Google Arts & Culture website. These are unofficial endpoints and can change; unreadable responses produce a retry option and never replace a valid cached copy. Directories use the server's pagination cursors. Museum searches return paginated matches; artist searches filter Google's matching topics, and place searches use Google's place suggestions. ⌘F focuses the directory's search field while browsing artists, museums or places; elsewhere it focuses artwork search.

Their responses and images are cached separately in `~/Library/Caches/Atelier/artsandculture`: directory results for a day, artist/museum/place/artwork details and images for a week, with an older copy available if a refresh fails. Settings includes their storage and can clear it. The development app uses its own cache. The native artwork viewer uses the image renditions Google provides; its website's gigapixel tile viewer is not embedded.

Favorites are saved in `~/Library/Application Support/Atelier/favorites.json`. The development app, from `bun run dev`, keeps its own in `Atelier Dev`.

## Testing the running app

`automation_e2e.go` adds mygo's automation plugin to builds with the `e2e` tag, so that another process can read the app's elements, press them and take screenshots:

```sh
go build -tags e2e -o /tmp/atelier-e2e . && MYGO_AUTOMATION=/tmp/atelier.sock /tmp/atelier-e2e
```

Its client is `mygo-automation`, in mygo's `plugins/automation/cmd/mygo-automation`.

## The collection

The Met artworks, pictures and their data come from its Open Access program, which makes images of public domain works free to use under [Creative Commons Zero](https://creativecommons.org/publicdomain/zero/1.0/). Museum and place previews come from Google Arts & Culture and retain their original owners' rights; they are not covered by The Met's CC0 license. Place descriptions link to their credited source. Atelier is an independent client and is not affiliated with or endorsed by The Met or Google.
