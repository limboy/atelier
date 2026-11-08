# Atelier

A native desktop app for exploring art from [The Metropolitan Museum of Art](https://www.metmuseum.org/) and [Google Arts & Culture](https://artsandculture.google.com/). Built in Go with [mygo](https://github.com/egoist/mygo).

- Discover featured artworks, themed collections and art across time.
- Browse and search artists, museums and places.
- Explore high-resolution artworks with zoom, pan and full screen.
- Save favorite artworks and clear cached content in Settings.

## Development

Requires Go 1.27.3 or later and [Bun](https://bun.sh).

```sh
bun install
bun run dev
```

The app rebuilds and restarts when files change.

## Build

```sh
bun run build      # app and DMG in build/
bun run build:dmg  # verified macOS DMG for Apple Silicon and Intel
```

Open the generated DMG and drag Atelier to Applications.

For signed and notarized builds, set `APPLE_SIGNING_IDENTITY` and `APPLE_NOTARY_PROFILE`, with an optional `APPLE_KEYCHAIN_PATH`. Local builds use ad hoc signing by default. See [mygo.config.ts](mygo.config.ts).

## Tests

```sh
go test .
```

To also check the live Google Arts & Culture endpoints:

```sh
ATELIER_LIVE_TEST=1 go test -run 'TestCulture.*Live' -v .
```

## Controls

Zoom with the trackpad, mouse wheel or a double click; drag to pan. Use `⌘=` / `⌘−` to zoom and `⌘0` to reset. Press `F` for full screen and `Esc` to leave it or go back. `⌘F` focuses directory search, and `⌘,` opens Settings.

## Data and credits

Content is cached in `~/Library/Caches/Atelier/` and can be cleared in Settings. Favorites are saved in `~/Library/Application Support/Atelier/favorites.json`. Development builds keep separate data.

The app uses [The Met's Open Access API](https://metmuseum.github.io/) and unofficial Google Arts & Culture endpoints, which may change. Cached content remains available when refreshes fail.

The Met's public domain images and Open Access data are available under [CC0](https://creativecommons.org/publicdomain/zero/1.0/). Google Arts & Culture content retains its owners' rights. Atelier is independent and is not affiliated with or endorsed by The Met or Google.
