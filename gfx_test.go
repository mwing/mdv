package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeImages(t *testing.T) string {
	dir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 400, 200))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 7)
	}
	var p, j bytes.Buffer
	png.Encode(&p, img)
	jpeg.Encode(&j, img, nil)
	os.WriteFile(filepath.Join(dir, "a b.png"), p.Bytes(), 0o644)
	os.WriteFile(filepath.Join(dir, "c.jpg"), j.Bytes(), 0o644)
	os.WriteFile(filepath.Join(dir, "bad.png"), []byte("not an image"), 0o644)
	return dir
}

func TestImages(t *testing.T) {
	g := newGfx(10, 20, 40, writeImages(t))
	r := &renderer{g: &unicodeGlyphs, color: true, gfx: g}
	src := "before ![x](a%20b.png) after\n\n- <img src=\"c.jpg\" alt=\"y\">\n\n" +
		"![z](bad.png) ![r](https://x.y/z.png) ![](missing.png)\n\n# ![h](c.jpg)"
	got := r.document(src, 30)

	// 400x200 px at 10x20 px cells is 40x10 cells, scaled to the 30 columns.
	row := func(id uint32, y, cols int) string {
		return (&picture{id: id, cols: cols, rows: y + 1}).lines()[y]
	}
	first := g.ids[0]
	want := []string{"before"}
	for y := 0; y < 8; y++ {
		want = append(want, row(first, y, 30))
	}
	want = append(want, "after", "")
	for y := 0; y < 7; y++ {
		pad := "  "
		if y == 0 {
			pad = "\x1b[36m•\x1b[0m "
		}
		want = append(want, pad+row(first+1, y, 28))
	}
	want = append(want, "",
		"\x1b[2m[image: z]\x1b[0m \x1b[2m[image: r]\x1b[0m \x1b[2m[image]\x1b[0m", "",
		"\x1b[1;2;35m[image: h]\x1b[0m", "\x1b[35m══════════\x1b[0m")
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("line %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
	if len(g.ids) != 2 {
		t.Fatalf("transmitted %d images, want 2", len(g.ids))
	}
	sent := g.buf.Len()
	g.limit = sent + 100
	if over := r.document("![x](c.jpg)", 20); len(over) != 1 || g.buf.Len() != sent || len(g.ids) != 2 {
		t.Errorf("image over the limit was sent: %q", over)
	}
	g.limit = maxImageTotal
	if under := r.document("![x](c.jpg)", 25); len(under) < 2 || g.buf.Len() <= sent || len(g.ids) != 3 {
		t.Errorf("image under the limit was not sent: %q", under)
	}
	sent = g.buf.Len()
	if again := r.document("![x](a%20b.png)", 30); len(again) != 8 || g.buf.Len() != sent {
		t.Errorf("repeated image: %d lines, %d new bytes", len(again), g.buf.Len()-sent)
	}

	cmds := strings.Split(strings.TrimSuffix(g.buf.String(), "\x1b\\"), "\x1b\\")
	starts := 0
	for i, c := range cmds {
		head, payload, ok := strings.Cut(c, ";")
		if !ok || !strings.HasPrefix(head, "\x1b_G") || len(payload) > 4096 {
			t.Fatalf("command %d malformed: %.60q", i, c)
		}
		last := i == len(cmds)-1 || strings.Contains(cmds[i+1], "a=T")
		if strings.HasSuffix(head, "m=0") != last {
			t.Errorf("command %d: wrong continuation flag in %q", i, head)
		}
		if !last && len(payload)%4 != 0 {
			t.Errorf("command %d: chunk of %d bytes is not a multiple of 4", i, len(payload))
		}
		if strings.Contains(head, "a=T") {
			starts++
			if !strings.Contains(head, "U=1") || !strings.Contains(head, "q=2") {
				t.Errorf("command %d: %q", i, head)
			}
		} else if head != "\x1b_Gq=2,m=0" && head != "\x1b_Gq=2,m=1" {
			t.Errorf("command %d: continuation carries extra keys: %q", i, head)
		}
	}
	if starts != 3 {
		t.Errorf("%d transmissions, want 3", starts)
	}
}

func TestProbe(t *testing.T) {
	calls := 0
	g := newGfx(10, 20, 40, writeImages(t))
	g.probe = func() bool { calls++; return false }
	r := &renderer{g: &unicodeGlyphs, color: true, gfx: g}
	got := strings.Join(r.document("![x](c.jpg)\n\n![y](c.jpg)\n\ntext", 30), "|")
	if want := "\x1b[2m[image: x]\x1b[0m||\x1b[2m[image: y]\x1b[0m||text"; got != want || calls != 1 || g.buf.Len() != 0 {
		t.Errorf("got %q, %d probes, %d bytes sent", got, calls, g.buf.Len())
	}

	for reply, want := range map[string]string{
		"\x1bP>|ghostty 1.3.1\x1b\\\x1b[?62;22c": "ghostty 1.3.1",
		"\x1b[?62;22c":                           "",
		"\x1bP>|xterm.js(5.5.0)\x1b\\":           "xterm.js(5.5.0)",
		"\x1bP>|kitty(0.42.1)":                   "",
	} {
		if got := parseXTVersion(reply); got != want {
			t.Errorf("parseXTVersion(%q) = %q, want %q", reply, got, want)
		}
	}
	for name, want := range map[string]bool{"ghostty 1.3.1": true, "kitty(0.42.1)": true, "xterm.js(5.5.0)": false, "": false} {
		if hasPlaceholders(name) != want {
			t.Errorf("hasPlaceholders(%q) != %v", name, want)
		}
	}
}

func TestPlaceholderRow(t *testing.T) {
	got := (&picture{id: 0x010203, cols: 3, rows: 2}).lines()
	want := "\x1b[38;2;1;2;3m\U0010EEEE̍̅\U0010EEEE\U0010EEEE\x1b[0m"
	if got[1] != want {
		t.Errorf("got %q\nwant %q", got[1], want)
	}
}

func TestFit(t *testing.T) {
	g := newGfx(10, 20, 22, "")
	for _, tt := range []struct{ w, h, max, cols, rows int }{
		{95, 30, 80, 10, 2},
		{1600, 400, 80, 80, 10},
		{400, 4000, 80, 4, 20},
		{1, 1, 80, 1, 1},
	} {
		if c, r := g.fit(tt.w, tt.h, tt.max); c != tt.cols || r != tt.rows {
			t.Errorf("fit(%d,%d,%d) = %d,%d; want %d,%d", tt.w, tt.h, tt.max, c, r, tt.cols, tt.rows)
		}
	}
}

func TestShrink(t *testing.T) {
	img := image.NewRGBA(image.Rect(5, 5, 13, 9))
	for y := 5; y < 9; y++ {
		for x := 5; x < 13; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8((x - 5) / 2 * 10), 200, 0, 255})
		}
	}
	got := shrink(img, 4, 2)
	if got.Rect.Dx() != 4 || got.Rect.Dy() != 2 {
		t.Fatalf("size %v", got.Rect)
	}
	if c := got.RGBAAt(3, 1); c != (color.RGBA{30, 200, 0, 255}) {
		t.Errorf("pixel %v", c)
	}
	if same := shrink(img, 8, 4); same.Rect.Dx() != 8 {
		t.Errorf("unexpected resize to %v", same.Rect)
	}
}
