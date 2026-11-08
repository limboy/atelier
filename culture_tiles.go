package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Google serves a separate tile pyramid for its artwork viewer. The page
// supplies the image address and a public signing token; =g describes the
// levels. Protocol reference: https://github.com/lovasoa/dezoomify/tree/main/crates/dezoomify/src/google_arts_and_culture
// Keep this separate from cultureImage: resizing a preview never requests tiles.
type culturePyramid struct {
	base, token           string
	tileWidth, tileHeight int
	levels                []tileLevel
}

type tileLevel struct{ width, height, cols, rows int }
type tileCoord struct{ z, x, y int }

var cultureTilePage = regexp.MustCompile(`\]\s*,\s*"(//[a-zA-Z0-9./_:\-]+)"\s*,\s*(?:"([^"]*)"|null)`)
var cultureImageHost = regexp.MustCompile(`^lh[0-9]+\.(googleusercontent\.com|ggpht\.com)$`)

func parseCultureTilePage(b []byte) (*culturePyramid, error) {
	m := cultureTilePage.FindSubmatch(b)
	if m == nil {
		return nil, errors.New("artwork has no tile information")
	}
	u, err := url.Parse("https:" + string(m[1]))
	if err != nil || u.User != nil || u.Port() != "" || !cultureImageHost.MatchString(u.Hostname()) || u.Path == "" || strings.Contains(u.Path, "=") {
		return nil, errors.New("invalid artwork tile address")
	}
	return &culturePyramid{base: u.String(), token: string(m[2])}, nil
}

func (p *culturePyramid) readLevels(b []byte) error {
	var info struct {
		XMLName xml.Name `xml:"TileInfo"`
		Width   int      `xml:"tile_width,attr"`
		Height  int      `xml:"tile_height,attr"`
		Origin  string   `xml:"origin,attr"`
		Levels  []struct {
			Cols   int `xml:"num_tiles_x,attr"`
			Rows   int `xml:"num_tiles_y,attr"`
			EmptyX int `xml:"empty_pels_x,attr"`
			EmptyY int `xml:"empty_pels_y,attr"`
		} `xml:"pyramid_level"`
	}
	if err := xml.Unmarshal(b, &info); err != nil {
		return fmt.Errorf("invalid artwork tile information: %w", err)
	}
	if info.Width <= 0 || info.Height <= 0 || info.Width > 2048 || info.Height > 2048 ||
		len(info.Levels) == 0 || len(info.Levels) > 32 || info.Origin != "" && info.Origin != "TOP_LEFT" {
		return errors.New("unsupported artwork tile pyramid")
	}
	levels := make([]tileLevel, 0, len(info.Levels))
	for _, l := range info.Levels {
		if l.Cols <= 0 || l.Rows <= 0 || l.Cols > 1<<20 || l.Rows > 1<<20 ||
			l.EmptyX < 0 || l.EmptyX >= info.Width || l.EmptyY < 0 || l.EmptyY >= info.Height {
			return errors.New("invalid artwork tile dimensions")
		}
		level := tileLevel{info.Width*l.Cols - l.EmptyX, info.Height*l.Rows - l.EmptyY, l.Cols, l.Rows}
		if n := len(levels); n > 0 && (level.width < levels[n-1].width || level.height < levels[n-1].height) {
			return errors.New("artwork tile levels are out of order")
		}
		levels = append(levels, level)
	}
	p.tileWidth, p.tileHeight, p.levels = info.Width, info.Height, levels
	return nil
}

func (g *Culture) Tiles(ctx context.Context, id string) (*culturePyramid, error) {
	if !validCultureAssetID(id) {
		return nil, errors.New("invalid artwork ID")
	}
	b, err := g.get(ctx, cultureBase+"/asset/"+id+"?hl=en", 24*time.Hour, func(b []byte) error {
		_, err := parseCultureTilePage(b)
		return err
	})
	if err != nil {
		return nil, err
	}
	p, err := parseCultureTilePage(b)
	if err != nil {
		return nil, err
	}
	_, err = g.get(ctx, p.base+"=g", 7*24*time.Hour, p.readLevels)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *culturePyramid) tileSize(c tileCoord) (int, int) {
	if c.z < 0 || c.z >= len(p.levels) {
		return 0, 0
	}
	l := p.levels[c.z]
	if c.x < 0 || c.y < 0 || c.x >= l.cols || c.y >= l.rows {
		return 0, 0
	}
	return min(p.tileWidth, l.width-c.x*p.tileWidth), min(p.tileHeight, l.height-c.y*p.tileHeight)
}

func (p *culturePyramid) tileURL(c tileCoord) string {
	suffix := fmt.Sprintf("=x%d-y%d-z%d-t", c.x, c.y, c.z)
	u, _ := url.Parse(p.base)
	mac := hmac.New(sha1.New, []byte{123, 43, 78, 35, 222, 44, 197, 197})
	_, _ = mac.Write([]byte(strings.TrimPrefix(u.Path, "/") + suffix + p.token))
	signature := strings.ReplaceAll(base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), "-", "_")
	return p.base + suffix + signature
}

// Tile validates before caching, including the decrypted image dimensions.
// Edge tiles may be padded to the regular tile size; the renderer crops them.
func (g *Culture) Tile(ctx context.Context, p *culturePyramid, c tileCoord) ([]byte, error) {
	w, h := p.tileSize(c)
	if w == 0 || h == 0 {
		return nil, errors.New("tile is outside the artwork")
	}
	b, err := g.get(ctx, p.tileURL(c), 7*24*time.Hour, func(b []byte) error {
		plain, err := decryptCultureTile(b)
		if err != nil {
			return err
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(plain))
		if err != nil {
			return err
		}
		if cfg.Width < w || cfg.Height < h || cfg.Width > p.tileWidth || cfg.Height > p.tileHeight {
			return errors.New("artwork tile has unexpected dimensions")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return decryptCultureTile(b)
}

// Some tiles are ordinary images. Others frame a plaintext image header,
// an AES-CBC encrypted middle, and a plaintext footer with the header's size.
func decryptCultureTile(b []byte) ([]byte, error) {
	if len(b) < 4 || binary.LittleEndian.Uint32(b[:4]) != 0x0a0a0a0a {
		return b, nil
	}
	if len(b) < 12 {
		return nil, errors.New("truncated artwork tile")
	}
	header := int64(binary.LittleEndian.Uint32(b[len(b)-4:]))
	if header > int64(len(b)-12) {
		return nil, errors.New("invalid artwork tile header")
	}
	start := 8 + int(header)
	n := int64(binary.LittleEndian.Uint32(b[start-4 : start]))
	if n == 0 || n%aes.BlockSize != 0 || n > int64(len(b)-4-start) {
		return nil, errors.New("invalid artwork tile encryption")
	}
	out := make([]byte, len(b)-12)
	copy(out, b[4:start-4])
	copy(out[header:], b[start:len(b)-4])
	block, err := aes.NewCipher([]byte{91, 99, 219, 17, 59, 122, 243, 224, 177, 67, 85, 86, 200, 249, 83, 12})
	if err != nil {
		return nil, err
	}
	iv := []byte{113, 231, 4, 5, 53, 58, 119, 139, 250, 111, 188, 48, 50, 27, 149, 146}
	chunk := out[header : header+n]
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(chunk, chunk)
	return out, nil
}
