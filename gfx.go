package main

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"math/rand/v2"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxImageBytes  = 32 << 20
	maxImagePixels = 64 << 20
	placeholder    = "\U0010EEEE"
)

type imageKey struct {
	path string
	cols int
}

type picture struct {
	id         uint32
	cols, rows int
}

// Images are placed with Unicode placeholders because those are ordinary
// coloured text and so survive a pager.
type gfx struct {
	cellW, cellH float64
	maxRows      int
	dir          string
	next         uint32
	ids          []uint32
	probe        func() bool
	off          bool
	cache        map[imageKey]*picture
	buf          bytes.Buffer
}

func newGfx(cellW, cellH float64, screenRows int, dir string) *gfx {
	return &gfx{
		cellW:   cellW,
		cellH:   cellH,
		maxRows: max(1, min(screenRows-2, len(diacritics))),
		dir:     dir,
		cache:   map[imageKey]*picture{},
		// Random start: images from an earlier run may still be on screen.
		next: rand.Uint32N(0xFF0000) + 1,
	}
}

func (g *gfx) fit(pw, ph, maxCols int) (cols, rows int) {
	w, h := float64(pw), float64(ph)
	scale := min(1, float64(maxCols)*g.cellW/w, float64(g.maxRows)*g.cellH/h)
	cols = max(1, min(maxCols, int(math.Ceil(w*scale/g.cellW))))
	rows = max(1, min(g.maxRows, int(math.Ceil(h*scale/g.cellH))))
	return cols, rows
}

func (g *gfx) send(keys string, payload []byte) {
	enc := base64.StdEncoding.EncodeToString(payload)
	for first := true; first || len(enc) > 0; first = false {
		n := min(4096, len(enc))
		g.buf.WriteString("\x1b_G")
		if first {
			g.buf.WriteString(keys)
			g.buf.WriteByte(',')
		} else {
			// Repeated so a terminal that loses track of the transfer stays silent.
			g.buf.WriteString("q=2,")
		}
		if n < len(enc) {
			g.buf.WriteString("m=1;")
		} else {
			g.buf.WriteString("m=0;")
		}
		g.buf.WriteString(enc[:n])
		g.buf.WriteString("\x1b\\")
		enc = enc[n:]
	}
}

func (g *gfx) load(src string, maxCols int) *picture {
	if src == "" || strings.Contains(src, "://") {
		return nil
	}
	if p, err := url.PathUnescape(src); err == nil {
		src = p
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(g.dir, src)
	}
	key := imageKey{src, maxCols}
	p, seen := g.cache[key]
	if !seen {
		p = g.transmit(src, maxCols)
		g.cache[key] = p
	}
	return p
}

func readImage(path string) []byte {
	// Stat before open: opening a FIFO blocks, and devices never end.
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	// The read is capped itself because procfs files report a size of zero.
	data, err := io.ReadAll(io.LimitReader(f, maxImageBytes+1))
	if err != nil || len(data) > maxImageBytes {
		return nil
	}
	return data
}

func (g *gfx) transmit(path string, maxCols int) *picture {
	data := readImage(path)
	if data == nil {
		return nil
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxImagePixels {
		return nil
	}
	if g.probe != nil {
		g.off = !g.probe()
		g.probe = nil
	}
	if g.off {
		return nil
	}
	p := &picture{id: g.next}
	p.cols, p.rows = g.fit(cfg.Width, cfg.Height, maxCols)
	keys := fmt.Sprintf("a=T,U=1,q=2,i=%d,c=%d,r=%d", p.id, p.cols, p.rows)
	if format == "png" {
		g.send(keys+",f=100", data)
	} else {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil
		}
		rgba := shrink(img, int(float64(p.cols)*g.cellW), int(float64(p.rows)*g.cellH))
		var z bytes.Buffer
		zw, _ := zlib.NewWriterLevel(&z, zlib.BestSpeed)
		zw.Write(rgba.Pix)
		zw.Close()
		g.send(fmt.Sprintf("%s,f=32,o=z,s=%d,v=%d", keys, rgba.Rect.Dx(), rgba.Rect.Dy()), z.Bytes())
	}
	g.next++
	g.ids = append(g.ids, p.id)
	return p
}

func (g *gfx) deleteAll() []byte {
	var b bytes.Buffer
	for _, id := range g.ids {
		fmt.Fprintf(&b, "\x1b_Ga=d,d=I,q=2,i=%d\x1b\\", id)
	}
	return b.Bytes()
}

// Reduces by a whole factor only, so the result can be up to twice w by h.
// Never enlarges.
func shrink(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Rect, img, b.Min, draw.Src)
	f := min(b.Dx()/max(w, 1), b.Dy()/max(h, 1))
	if f <= 1 {
		return src
	}
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx()/f, b.Dy()/f))
	n := uint32(f * f)
	for y := 0; y < dst.Rect.Dy(); y++ {
		for x := 0; x < dst.Rect.Dx(); x++ {
			var sum [4]uint32
			for dy := 0; dy < f; dy++ {
				row := src.Pix[(y*f+dy)*src.Stride+x*f*4:]
				for i := 0; i < f*4; i++ {
					sum[i&3] += uint32(row[i])
				}
			}
			o := dst.Pix[y*dst.Stride+x*4:]
			for i, s := range sum {
				o[i] = uint8(s / n)
			}
		}
	}
	return dst
}

func (p *picture) lines() []string {
	head := fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s", p.id>>16&255, p.id>>8&255, p.id&255, placeholder)
	// Cells without diacritics continue the row from the cell to their left.
	tail := string(diacritics[0]) + strings.Repeat(placeholder, p.cols-1) + "\x1b[0m"
	out := make([]string, p.rows)
	for y := range out {
		out[y] = head + string(diacritics[y]) + tail
	}
	return out
}
