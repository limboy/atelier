package main

import "github.com/egoist/mygo/ui"

// Line icons drawn for this app, 24×24, stroked in currentColor.
func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	icHome     = icon(`<path d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1h-4.5v-6h-5v6H5a1 1 0 0 1-1-1z"/>`)
	icArtist   = icon(`<path d="M12 3a9 9 0 1 0 0 18h1.2a2.3 2.3 0 0 0 1.6-3.9 1.3 1.3 0 0 1 .9-2.2H18a3 3 0 0 0 3-3c0-5-4-8.9-9-8.9z"/><circle cx="7.5" cy="10" r=".8"/><circle cx="11" cy="6.8" r=".8"/><circle cx="15.5" cy="7.5" r=".8"/><circle cx="7.5" cy="14.5" r=".8"/>`)
	icMuseum   = icon(`<path d="m3 9 9-6 9 6M4 10h16M5 10v8M10 10v8M14 10v8M19 10v8M3 19h18M2 22h20"/>`)
	icGlobe    = icon(`<circle cx="12" cy="12" r="9"/><ellipse cx="12" cy="12" rx="4" ry="9"/><path d="M3 12h18M5 6.5h14M5 17.5h14"/>`)
	icClock    = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>`)
	icHeart    = icon(`<path d="M12 20s-7.5-4.6-7.5-10.2A4.3 4.3 0 0 1 12 7a4.3 4.3 0 0 1 7.5 2.8C19.5 15.4 12 20 12 20z"/>`)
	icHeartOn  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><path fill="currentColor" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" d="M12 20s-7.5-4.6-7.5-10.2A4.3 4.3 0 0 1 12 7a4.3 4.3 0 0 1 7.5 2.8C19.5 15.4 12 20 12 20z"/></svg>`))
	icPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	icMinus    = icon(`<path d="M5 12h14"/>`)
	icExpand   = icon(`<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>`)
	icShrink   = icon(`<path d="M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5"/>`)
	icExternal = icon(`<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>`)
	icGear     = icon(`<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`)
	icTrash    = icon(`<path d="M4 7h16M10 11v6M14 11v6M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2l1-12M9 7V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3"/>`)
	icFolder   = icon(`<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>`)
	icLeft     = icon(`<path d="m15 5-7 7 7 7"/>`)
	icRight    = icon(`<path d="m9 5 7 7-7 7"/>`)
	icShuffle  = icon(`<path d="M4 7h3c5 0 5 10 10 10h3M4 17h3c1.6 0 2.6-1 3.4-2.3M13.6 9.3C14.4 8 15.4 7 17 7h3M18 4l3 3-3 3M18 14l3 3-3 3"/>`)
)
