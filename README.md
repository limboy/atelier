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

## Releases and updates

Pushing a tag builds a signed, notarized universal DMG on GitHub Actions and publishes it as a release, together with the signed files that installed copies update from. Atelier checks for a new release at launch, and from **Atelier › Check for Updates…**.

```sh
# 1. bump "version" in package.json, commit
# 2. tag it and push
git tag v0.2.0 && git push origin v0.2.0
```

[The workflow](.github/workflows/release.yml) needs these repository secrets: `MACOS_CERTIFICATE` (base64 of the Developer ID `.p12`), `MACOS_CERTIFICATE_PASSWORD`, `APPLE_SIGNING_IDENTITY`, `APPLE_API_KEY_P8` (base64), `APPLE_API_KEY_ID`, `APPLE_API_ISSUER_ID` and `MYGO_UPDATER_PRIVATE_KEY`. The last one signs updates, and its public key is in [mygo.config.ts](mygo.config.ts). Losing it strands installed copies, so keep a backup.

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
