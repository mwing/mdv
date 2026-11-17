package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const usage = `usage: mdv [options] [file]

Renders markdown in the terminal. Reads stdin when no file is given.

  -w N           wrap at N columns (default: terminal width, at most 100)
  -color WHEN    auto, always or never (default auto; NO_COLOR is honoured)
  -ascii         use ASCII glyphs only
  -images WHEN   auto, always or never (default auto: kitty and Ghostty)
  -P             do not page
  -e             open the file in $VISUAL or $EDITOR first, then render it

Long output goes through $MDV_PAGER, $PAGER or less.
`

// The only barrier against a file injecting terminal escapes: nothing
// downstream escapes document text. C1 controls arrive as C2 80..9F.
func clean(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	out := b[:0]
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case c == 0xc2 && i+1 < len(b) && b[i+1] >= 0x80 && b[i+1] <= 0x9f:
			i++
		case c >= 0x20 && c != 0x7f || c == '\n' || c == '\t':
			out = append(out, c)
		}
	}
	return string(out)
}

func main() {
	var (
		width     int
		color     string
		images    string
		ascii     bool
		noPager   bool
		editFirst bool
	)
	flag.IntVar(&width, "w", 0, "")
	flag.StringVar(&color, "color", "auto", "")
	flag.StringVar(&images, "images", "auto", "")
	flag.BoolVar(&ascii, "ascii", false, "")
	flag.BoolVar(&noPager, "P", false, "")
	flag.BoolVar(&editFirst, "e", false, "")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	when := func(s string) bool { return s == "auto" || s == "always" || s == "never" }
	if !when(color) || !when(images) || flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}

	var src []byte
	var err error
	name := flag.Arg(0)
	stdin := name == "" || name == "-"
	if stdin {
		if _, tty := termSize(0); tty || editFirst {
			flag.Usage()
			os.Exit(2)
		}
		src, err = io.ReadAll(os.Stdin)
	} else {
		if editFirst {
			if err := edit(name); err != nil {
				fmt.Fprintln(os.Stderr, "mdv: editor:", err)
				os.Exit(1)
			}
		}
		src, err = os.ReadFile(name)
		if editFirst && os.IsNotExist(err) {
			return
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv:", err)
		os.Exit(1)
	}

	ws, tty := termSize(1)
	cols, rows := int(ws.Col), int(ws.Row)
	switch {
	case !tty && width == 0:
		width = 80
	case !tty:
	case width == 0:
		width = min(cols, 100)
	default:
		width = min(cols, width)
	}

	r := &renderer{g: &unicodeGlyphs}
	r.color = color == "always" ||
		color == "auto" && tty && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	if ascii || !utf8Locale() {
		r.g = &asciiGlyphs
	}

	if tty && r.color && images != "never" && (images == "always" || imageTerminal() && ws.X > 0 && ws.Y > 0) {
		cellW, cellH := 8.0, 16.0
		if ws.X > 0 && ws.Y > 0 {
			cellW, cellH = float64(ws.X)/float64(cols), float64(ws.Y)/float64(rows)
		}
		dir := "."
		if !stdin {
			dir = filepath.Dir(name)
		}
		r.gfx = newGfx(cellW, cellH, rows, dir)
		if images == "auto" {
			r.gfx.probe = func() bool { return hasPlaceholders(terminalName()) }
		}
	}

	text := clean(src)
	lines := r.document(text, width)
	paged := tty && !noPager && len(lines) >= rows
	var args []string
	if paged {
		args = pagerArgs()
		if r.gfx != nil && len(r.gfx.ids) > 0 && images != "always" && !pagerShowsImages(args) {
			r.gfx = nil
			lines = r.document(text, width)
			paged = len(lines) >= rows
		}
	}
	if len(lines) == 0 {
		return
	}
	hasImages := r.gfx != nil && len(r.gfx.ids) > 0

	out := bufio.NewWriterSize(os.Stdout, 1<<16)
	defer out.Flush()
	if hasImages {
		out.Write(r.gfx.buf.Bytes())
		out.Flush()
	}
	data := []byte(strings.Join(lines, "\n") + "\n")
	if paged && page(args, data, hasImages) == nil {
		if hasImages {
			out.Write(r.gfx.deleteAll())
		}
		return
	}
	out.Write(data)
}
