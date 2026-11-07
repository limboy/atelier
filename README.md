# Atelier

A desktop app for exploring art, built with [mygo](https://github.com/egoist/mygo)'s native UI: no webview, drawn on the GPU from Go. It browses the public domain collection of [The Metropolitan Museum of Art](https://www.metmuseum.org/) through its [Open Access API](https://metmuseum.github.io/).

- **Home**: a featured carousel, an art of the day, and rows of themed stories (Impressionism, Van Gogh, ukiyo-e, Ancient Egypt, armor…) that scroll sideways.
- **Explore**: artists, movements, mediums and the Met's departments, each opening its works in a grid that loads more as it scrolls.
- **Time machine**: a slider and eras from 3000 BCE to 1950.
- **Artwork**: a viewer that zooms with the trackpad, the wheel, a double click or ⌘= ⌘− ⌘0, pans by dragging, and loads the full resolution scan as you zoom in. **F** shows the artwork alone, full screen; **Esc** leaves. Beside it, the details, its themes, more by the artist and from its department.
- **Search** (⌘F) and **Favorites**.

## Developing

Go 1.27.3 or later and [Bun](https://bun.sh).

```sh
bun install
bun run dev       # develop: rebuilds and restarts the app as files change
bun run build     # build/darwin-arm64/Atelier.app and a .dmg
bun run build:dmg # verified universal DMG for Apple Silicon and Intel (macOS only)
go test .         # offline: the API client through a fake transport
```

To install the build you made:

```sh
bun run build && ditto build/darwin-arm64/Atelier.app /Applications/Atelier.app
```

`mygo.config.ts` reads Apple signing and notarization settings from `APPLE_SIGNING_IDENTITY`, `APPLE_NOTARY_PROFILE` and `APPLE_KEYCHAIN_PATH`; without them, builds are signed ad hoc for this Mac.

## Code

- `main.go`: the app's state, theme, routes and window.
- `met.go`: the client of the Met's API, with its disk cache and pacing.
- `state.go`: feeds of a search's artworks, loaded a page at a time, and favorites.
- `images.go`: pictures downloaded, scaled and decoded off the main thread, newest first.
- `components.go`: the top bar, artwork cards, rows and the masonry grid.
- `home.go`, `explore.go`, `collection.go` (collections, favorites, the time machine), `artwork.go` (the viewer and details): the pages.
- `resources/icon.png`: the app's icon, rendered from `design/icon.svg` with `rsvg-convert -w 1024 -h 1024 design/icon.svg -o resources/icon.png`.

## Data and the API

The API's firewall turns away bursts of requests with a 403. The app paces its requests to about three a second, two at a time, and every request backs off for 20 seconds or more after a 403; loads you move away from, as years passed on the time machine's slider, are cancelled.

Responses are cached in `~/Library/Caches/Atelier/met`: pictures for good, artworks for a week and searches for a day. An expired copy is fetched again, and shown when that fails, offline or while the API turns the app away. The cache has no size limit yet; deleting the folder empties it.

Favorites are saved in `~/Library/Application Support/Atelier/favorites.json`. The development app, from `bun run dev`, keeps its own in `Atelier Dev`.

## Testing the running app

`automation_e2e.go` adds mygo's automation plugin to builds with the `e2e` tag, so that another process can read the app's elements, press them and take screenshots:

```sh
go build -tags e2e -o /tmp/atelier-e2e . && MYGO_AUTOMATION=/tmp/atelier.sock /tmp/atelier-e2e
```

Its client is `mygo-automation`, in mygo's `plugins/automation/cmd/mygo-automation`.

## The collection

The artworks, pictures and their data come from The Met's Open Access program, which makes images of public domain works free to use under [Creative Commons Zero](https://creativecommons.org/publicdomain/zero/1.0/). Atelier is an independent client; it is not affiliated with or endorsed by The Met.
