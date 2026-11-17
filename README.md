# mdv

A small, fast markdown viewer for the terminal. One static binary, no dependencies
beyond the Go standard library.

```sh
mdv README.md
git show HEAD:README.md | mdv
mdv -e notes.md
```

## Install

Needs Go 1.24 or newer. macOS and Linux only.

```sh
go build -ldflags='-s -w' -o ~/.local/bin/mdv .
```

## Options

| Flag | Meaning |
|------|---------|
| `-w N` | Wrap at N columns. Default is the terminal width, at most 100. |
| `-color WHEN` | `auto`, `always` or `never`. `NO_COLOR` and `TERM=dumb` turn colour off. |
| `-ascii` | ASCII glyphs only. Automatic when the locale is not UTF-8. |
| `-images WHEN` | `auto`, `always` or `never`. `auto` shows images in kitty and Ghostty. |
| `-P` | Do not page. |
| `-e` | Open the file in `$VISUAL` or `$EDITOR` (default `vi`) first, then render the result. |

With no file, or `-`, `mdv` reads stdin.

## Behaviour

- Colours come from the terminal's own 16-colour palette, so output follows your theme.
- Without colour (piped, `NO_COLOR`, `-color never`) the output is plain text: links
  become `text (url)`, inline code keeps its backticks, and headings below level two keep
  their `#` marks.
- Output longer than the screen goes through `$MDV_PAGER`, `$PAGER` or `less -R`. Mouse
  wheel scrolling is the terminal's alternate-scroll feature; if yours lacks it, set
  `MDV_PAGER='less --mouse'`.
- Links are OSC 8 hyperlinks, so the URL is clickable but not printed.
- Code blocks are never hard-wrapped, so a copied line stays one line.
- Control bytes in the input are dropped, so a file cannot send escape sequences to the
  terminal.

## Images

In kitty and Ghostty, local PNG, JPEG and GIF files are drawn inline with the kitty
graphics protocol, scaled down to fit the text width and one screen of height. They are
placed with Unicode placeholders, so they scroll with the text inside `less`.

- Paths resolve relative to the markdown file. Remote URLs are never fetched.
- Anything that cannot be shown falls back to `[image: alt]`: remote and missing files,
  SVG and WebP, images in headings and table cells, tmux, zellij, and piped output.
- Before sending the first image, `mdv` asks the terminal for its name, because `TERM`
  is inherited by multiplexers and embedded terminals that cannot draw images.
- Paging images needs `less` 632 or newer. With an older `less` or another pager, long
  documents fall back to alt text; `-images always` overrides both checks.
- GIFs show their first frame.

## Supported markdown

Headings (ATX and setext), emphasis, strikethrough, inline code, links (inline, reference,
autolink, bare URL), images, fenced and indented code, block quotes, GitHub alerts, nested
lists, task lists, tables with alignment and cell wrapping, rules, front matter, HTML
entities, and common inline HTML tags (dropped, keeping their text).

Not supported: syntax highlighting and footnotes. The parser is pragmatic rather than a
full CommonMark implementation, so unusual emphasis nesting can render slightly off.

## Code layout

| File | Contents |
|------|----------|
| `main.go` | Flags, input, and the choice between direct output and the pager. |
| `render.go` | Block parsing and rendering: headings, lists, quotes, code, tables. |
| `inline.go` | Inline parsing: emphasis, code spans, links, images, HTML. |
| `text.go` | Styles, display width, and word wrapping. |
| `gfx.go` | Image loading, scaling, and kitty graphics commands. |
| `term.go` | Terminal size, locale, and terminal identification. |
| `pager.go` | Pager and editor processes. |
| `diacritics.go` | Row and column marks for image placeholders. |

Rendering is recursive: a container (quote, list item) renders its content at a narrower
width and prefixes each resulting line.

## Development

```sh
go test ./...
go test -bench . -run '^$'
go run . testdata/sample.md
```

`testdata/sample.md` exercises every supported construct.

## License

MIT. See [LICENSE](LICENSE).
