package main

import "github.com/egoist/mygo/ui"

// Line icons drawn for this app, 24×24, stroked in currentColor.
func icon(body string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">` + body + `</svg>`))
}

var (
	icHome     = icon(`<path d="M4 10.5 12 4l8 6.5V20a1 1 0 0 1-1 1h-4.5v-6h-5v6H5a1 1 0 0 1-1-1z"/>`)
	icCompass  = icon(`<circle cx="12" cy="12" r="8.5"/><path d="m15.5 8.5-2 5-5 2 2-5z"/>`)
	icClock    = icon(`<circle cx="12" cy="12" r="8.5"/><path d="M12 7.5V12l3 2"/>`)
	icHeart    = icon(`<path d="M12 20s-7.5-4.6-7.5-10.2A4.3 4.3 0 0 1 12 7a4.3 4.3 0 0 1 7.5 2.8C19.5 15.4 12 20 12 20z"/>`)
	icHeartOn  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="24" height="24"><path fill="currentColor" stroke="currentColor" stroke-width="1.8" stroke-linejoin="round" d="M12 20s-7.5-4.6-7.5-10.2A4.3 4.3 0 0 1 12 7a4.3 4.3 0 0 1 7.5 2.8C19.5 15.4 12 20 12 20z"/></svg>`))
	icPlus     = icon(`<path d="M12 5v14M5 12h14"/>`)
	icMinus    = icon(`<path d="M5 12h14"/>`)
	icExpand   = icon(`<path d="M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5"/>`)
	icShrink   = icon(`<path d="M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5"/>`)
	icExternal = icon(`<path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5"/>`)
	icLeft     = icon(`<path d="m15 5-7 7 7 7"/>`)
	icRight    = icon(`<path d="m9 5 7 7-7 7"/>`)
	icShuffle  = icon(`<path d="M4 7h3c5 0 5 10 10 10h3M4 17h3c1.6 0 2.6-1 3.4-2.3M13.6 9.3C14.4 8 15.4 7 17 7h3M18 4l3 3-3 3M18 14l3 3-3 3"/>`)
	// The app's mark: a frame around a sun over hills.
	logoMark = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32"><rect x="2" y="2" width="28" height="28" rx="7" fill="#b4472b"/><circle cx="20.5" cy="11.5" r="3.2" fill="#f6d9a8"/><path d="M5 24.5 12 15l5 6.2 3-3.4 7 6.7z" fill="#fbf4e8"/></svg>`))
)
