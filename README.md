# mdv

A small, fast markdown viewer for the terminal. One static binary, no dependencies
beyond the Go standard library.

```sh
mdv README.md
git show HEAD:README.md | mdv
mdv -e notes.md
```

## Install

macOS and Linux, arm64 and amd64.

Download a prebuilt binary from the [latest release](https://github.com/mwing/mdv/releases/latest).
This puts it in `~/.local/bin`, which must be on your `PATH`:

```sh
os=$(uname -s | tr A-Z a-z)
arch=$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')
mkdir -p ~/.local/bin
curl -fsSL "https://github.com/mwing/mdv/releases/latest/download/mdv_${os}_${arch}.tar.gz" |
  tar -xz -C ~/.local/bin mdv
```

Or, with Go 1.24 or newer:

```sh
go install github.com/mwing/mdv@latest
```

To check the download against the release's `checksums.txt` first, using `os` and `arch`
from above (on Linux without `shasum`, use `sha256sum -c -`):

```sh
(
  cd "$(mktemp -d)" &&
  base=https://github.com/mwing/mdv/releases/latest/download &&
  curl -fsSLO "$base/mdv_${os}_${arch}.tar.gz" -O "$base/checksums.txt" &&
  grep "mdv_${os}_${arch}.tar.gz" checksums.txt | shasum -a 256 -c - &&
  mkdir -p ~/.local/bin &&
  tar -xzf "mdv_${os}_${arch}.tar.gz" -C ~/.local/bin mdv
)
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
| `-version` | Print the version. |

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
- Input larger than 64 MiB is refused.

## Images

In kitty and Ghostty, local PNG, JPEG and GIF files are drawn inline with the kitty
graphics protocol, scaled down to fit the text width and one screen of height. They are
placed with Unicode placeholders, so they scroll with the text inside `less`.

- Paths resolve relative to the markdown file. Remote URLs are never fetched.
- Image paths are not confined to the document's directory, so `../assets/logo.png`
  works, and so does an absolute path. A document you do not trust can therefore put any
  image file you can read on your screen; use `-images never` for those.
- One document can send at most 64 MiB of image data; an image that would exceed that
  falls back to alt text.
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

Not supported: syntax highlighting, footnotes, and links inside link text. Quotes and
lists nested more than 32 deep are shown as plain text. The parser is pragmatic rather than a
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

Pushing a version tag such as `v0.1.0` runs `.github/workflows/release.yml`, which tests, builds the four
binaries and publishes them as a GitHub release.

## License

MIT. See [LICENSE](LICENSE).
