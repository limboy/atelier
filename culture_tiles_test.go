package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func testTileJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			m.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 11), B: 90, A: 255})
		}
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, m, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func testEncryptedTile(t *testing.T, plain []byte) []byte {
	t.Helper()
	const header, size = 32, 64
	block, err := aes.NewCipher([]byte{91, 99, 219, 17, 59, 122, 243, 224, 177, 67, 85, 86, 200, 249, 83, 12})
	if err != nil {
		t.Fatal(err)
	}
	iv := []byte{113, 231, 4, 5, 53, 58, 119, 139, 250, 111, 188, 48, 50, 27, 149, 146}
	var b bytes.Buffer
	b.Write([]byte{10, 10, 10, 10})
	b.Write(plain[:header])
	_ = binary.Write(&b, binary.LittleEndian, uint32(size))
	encrypted := make([]byte, size)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain[header:header+size])
	b.Write(encrypted)
	b.Write(plain[header+size:])
	_ = binary.Write(&b, binary.LittleEndian, uint32(header))
	return b.Bytes()
}

func TestCultureTileDiscoveryAndSigning(t *testing.T) {
	base := "https://lh3.googleusercontent.com/wGcDNN8L-2COcm9toX5BTp6HPxpMPPPuxrMU-ZL-W-nDHW8I_L4R5vlBJ6ITtlmONQ"
	p, err := parseCultureTilePage([]byte(`],"` + strings.TrimPrefix(base, "https:") + `","KwCgJ1QIfgprHn0a93x7Q-HhJ04"`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.tileURL(tileCoord{7, 0, 0}); got != base+"=x0-y0-z7-tHeJ3xylnSyyHPGwMZimI4EV3JP8" {
		t.Fatalf("wrong tile signature: %s", got)
	}
	for _, page := range []string{
		"],\n\"//lh5.ggpht.com/ci/image\",null",
		"],\"//lh3.googleusercontent.com/ci/image\",\"\"",
	} {
		if _, err := parseCultureTilePage([]byte(page)); err != nil {
			t.Fatal(err)
		}
	}
	for _, page := range []string{
		`<html>no tiles</html>`,
		`],"//example.com/image","token"`,
		`],"//lh3.googleusercontent.com.example.com/image","token"`,
		`],"//lh3.googleusercontent.com:8080/image","token"`,
	} {
		if _, err := parseCultureTilePage([]byte(page)); err == nil {
			t.Fatalf("invalid tile page accepted: %s", page)
		}
	}
}

const testTileXML = `<TileInfo tile_width="16" tile_height="16" origin="TOP_LEFT"><pyramid_level num_tiles_x="1" num_tiles_y="1" empty_pels_x="0" empty_pels_y="4"/><pyramid_level num_tiles_x="2" num_tiles_y="2" empty_pels_x="2" empty_pels_y="8"/></TileInfo>`

func TestCultureTileLevelsAndMalformedMetadata(t *testing.T) {
	p := &culturePyramid{}
	if err := p.readLevels([]byte(testTileXML)); err != nil {
		t.Fatal(err)
	}
	if p.levels[1].width != 30 || p.levels[1].height != 24 {
		t.Fatalf("wrong dimensions: %+v", p.levels)
	}
	if w, h := p.tileSize(tileCoord{1, 1, 1}); w != 14 || h != 8 {
		t.Fatalf("edge size %d×%d, want 14×8", w, h)
	}
	for _, c := range []tileCoord{{-1, 0, 0}, {2, 0, 0}, {1, -1, 0}, {1, 2, 0}, {1, 0, 2}} {
		if w, h := p.tileSize(c); w != 0 || h != 0 {
			t.Fatalf("out-of-bounds tile accepted: %+v", c)
		}
	}
	for _, bad := range []string{
		`<html/>`,
		strings.Replace(testTileXML, `tile_width="16"`, `tile_width="0"`, 1),
		strings.Replace(testTileXML, `tile_width="16"`, `tile_width="100000"`, 1),
		strings.Replace(testTileXML, `empty_pels_x="2"`, `empty_pels_x="16"`, 1),
		strings.Replace(testTileXML, `num_tiles_x="2"`, `num_tiles_x="0"`, 1),
		strings.Replace(testTileXML, `TOP_LEFT`, `BOTTOM_LEFT`, 1),
		strings.Replace(testTileXML, `num_tiles_x="1"`, `num_tiles_x="3"`, 1),
	} {
		if err := new(culturePyramid).readLevels([]byte(bad)); err == nil {
			t.Fatalf("invalid metadata accepted: %s", bad)
		}
	}
}

func TestCultureTileDecryption(t *testing.T) {
	plain := testTileJPEG(t, 16, 16)
	encoded := testEncryptedTile(t, plain)
	for _, b := range [][]byte{plain, encoded} {
		got, err := decryptCultureTile(b)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("image changed while decrypting: %v", err)
		}
	}
	badHeader := bytes.Clone(encoded)
	binary.LittleEndian.PutUint32(badHeader[len(badHeader)-4:], ^uint32(0))
	badSize := bytes.Clone(encoded)
	binary.LittleEndian.PutUint32(badSize[36:], 17)
	oversize := bytes.Clone(encoded)
	binary.LittleEndian.PutUint32(oversize[36:], 1<<30)
	for _, b := range [][]byte{{10, 10, 10, 10}, encoded[:10], badHeader, badSize, oversize} {
		if _, err := decryptCultureTile(b); err == nil {
			t.Fatal("malformed encrypted tile accepted")
		}
	}
}

func TestCultureTileDownloadsCacheAndCropEdges(t *testing.T) {
	g := NewCulture(t.TempDir())
	plain := testTileJPEG(t, 16, 16)
	encoded := testEncryptedTile(t, plain)
	var requests int
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		body := encoded
		switch {
		case r.URL.Host == "artsandculture.google.com":
			body = []byte(`],"//lh5.ggpht.com/ci/test","token"`)
		case strings.HasSuffix(r.URL.Path, "=g"):
			body = []byte(testTileXML)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
	})
	ctx := context.Background()
	p, err := g.Tiles(ctx, "artwork")
	if err != nil {
		t.Fatal(err)
	}
	a := &app{culture: g}
	s := newArtworkTiles(a)
	for range 2 {
		tile, err := s.load(ctx, p, tileCoord{1, 1, 1})
		if err != nil || tile.w != 14 || tile.h != 8 {
			t.Fatalf("padded edge tile not cropped: %+v, %v", tile, err)
		}
	}
	if requests != 3 {
		t.Fatalf("%d requests, want page, metadata and one cached tile", requests)
	}
	if _, err := g.Tiles(ctx, "artwork"); err != nil || requests != 3 {
		t.Fatalf("metadata cache not reused: %d requests, %v", requests, err)
	}
	if _, err := g.Tile(ctx, p, tileCoord{1, 2, 0}); err == nil || requests != 3 {
		t.Fatal("invalid coordinate made a network request")
	}
}

func TestCultureInvalidTileNeverPoisonsCache(t *testing.T) {
	g := NewCulture(t.TempDir())
	p := &culturePyramid{base: "https://lh3.googleusercontent.com/ci/test", tileWidth: 16, tileHeight: 16, levels: []tileLevel{{16, 16, 1, 1}}}
	valid := testTileJPEG(t, 16, 16)
	for _, invalid := range [][]byte{[]byte("<html>try later</html>"), testTileJPEG(t, 32, 32), testTileJPEG(t, 8, 8)} {
		g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(invalid))}, nil
		})
		if _, err := g.Tile(context.Background(), p, tileCoord{}); err == nil {
			t.Fatal("invalid tile accepted")
		}
		if _, err := os.Stat(g.cachePath(p.tileURL(tileCoord{}))); !os.IsNotExist(err) {
			t.Fatal("invalid tile was cached")
		}
	}
	g.http.Transport = cultureTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(valid))}, nil
	})
	if _, err := g.Tile(context.Background(), p, tileCoord{}); err != nil {
		t.Fatalf("valid retry failed: %v", err)
	}
}

func TestCultureTilesLive(t *testing.T) {
	if os.Getenv("ATELIER_LIVE_TEST") != "1" {
		t.Skip("set ATELIER_LIVE_TEST=1 to check live artwork tiles")
	}
	for _, id := range []string{"GgHsT2RumWxbtw", "dAFXSL9sZ1ulDw"} {
		t.Run(id, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			g := NewCulture(t.TempDir())
			p, err := g.Tiles(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			z := len(p.levels) - 1
			l := p.levels[z]
			for _, c := range []tileCoord{{z, l.cols / 2, l.rows / 2}, {z, l.cols - 1, l.rows - 1}} {
				s := newArtworkTiles(&app{culture: g})
				tile, err := s.load(ctx, p, c)
				if err != nil {
					t.Fatal(err)
				}
				w, h := p.tileSize(c)
				if tile.w != w || tile.h != h {
					t.Fatalf("wrong decoded tile size: %+v", tile)
				}
			}
			t.Log(fmt.Sprintf("%d levels, %d×%d scan: center and edge tiles decoded", len(p.levels), l.width, l.height))
		})
	}
}
