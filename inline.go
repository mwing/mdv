package main

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	codeStyle = style{fg: yellow}
	linkStyle = style{attr: underline, fg: blue}
	emph      = [...]style{{}, {attr: italic}, {attr: bold}, {attr: bold | italic}}
)

var entities = map[string]rune{
	"amp": '&', "lt": '<', "gt": '>', "quot": '"', "apos": '\'', "nbsp": 0xA0,
	"copy": '©', "reg": '®', "trade": '™', "mdash": '—', "ndash": '–', "hellip": '…',
	"larr": '←', "rarr": '→', "times": '×', "middot": '·', "laquo": '«', "raquo": '»', "deg": '°',
}

// true marks tags that end a line of text.
var htmlTags = map[string]bool{
	"a": false, "abbr": false, "b": false, "code": false, "del": false, "em": false, "i": false,
	"img": false, "ins": false, "kbd": false, "picture": false, "s": false, "samp": false,
	"small": false, "source": false, "span": false, "strong": false, "sub": false, "sup": false,
	"u": false, "video": false,
	"blockquote": true, "br": true, "center": true, "dd": true, "details": true, "div": true,
	"dl": true, "dt": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"hr": true, "li": true, "ol": true, "p": true, "pre": true, "summary": true, "table": true,
	"tbody": true, "td": true, "th": true, "thead": true, "tr": true, "ul": true,
}

func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

func findRun(s string, i int, c byte, n int) int {
	for i < len(s) {
		if s[i] != c {
			i++
			continue
		}
		m := run(s, i, c)
		if m == n {
			return i
		}
		i += m
	}
	return -1
}

func skipCode(s string, k int) int {
	n := run(s, k, '`')
	if e := findRun(s, k+n, '`', n); e >= 0 {
		return e + n - 1
	}
	return k + n - 1
}

func isPunct(c byte) bool {
	return c > 0x20 && c < 0x7f && !isAlnum(c)
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c|0x20 >= 'a' && c|0x20 <= 'z'
}

func flank(s string, i, n int) (open, close bool) {
	prev, next := ' ', ' '
	if i > 0 {
		prev, _ = utf8.DecodeLastRuneInString(s[:i])
	}
	if i+n < len(s) {
		next, _ = utf8.DecodeRuneInString(s[i+n:])
	}
	ps, ns := unicode.IsSpace(prev), unicode.IsSpace(next)
	pp := unicode.IsPunct(prev) || unicode.IsSymbol(prev)
	np := unicode.IsPunct(next) || unicode.IsSymbol(next)
	left := !ns && (!np || ps || pp)
	right := !ps && (!pp || ns || np)
	if s[i] == '_' {
		return left && (!right || pp), right && (!left || np)
	}
	return left, right
}

// For a closing run longer than use, the result is its last use bytes, not its start.
func findCloser(s string, from int, c byte, use int, exact bool) int {
	for k := from; k < len(s); k++ {
		switch s[k] {
		case '\\':
			k++
		case '`':
			k = skipCode(s, k)
		case c:
			m := run(s, k, c)
			if _, cl := flank(s, k, m); cl && (m == use || !exact && m > use) {
				return k + m - use
			}
			k += m - 1
		}
	}
	return -1
}

// The cache keeps nested openers linear; a fresh scan per opener is
// quadratic. -1 marks an opener with no closer. An entry depends only on the
// text after its opener, so it stays valid however the caller got there.
func match(s string, i int, open, close byte, code bool, cache map[int]int) int {
	if j, ok := cache[i]; ok {
		return j
	}
	var stack []int
scan:
	for k := i; k < len(s); k++ {
		switch c := s[k]; {
		case c == '\\':
			k++
		case c == '`' && code:
			k = skipCode(s, k)
		case c == open:
			if j, ok := cache[k]; ok {
				// An inner opener that never closes leaves no closer for
				// the ones around it.
				if j < 0 {
					break scan
				}
				k = j
				continue
			}
			stack = append(stack, k)
		case c == close:
			n := len(stack)
			cache[stack[n-1]] = k
			stack = stack[:n-1]
			if n == 1 {
				return k
			}
		}
	}
	for _, p := range stack {
		cache[p] = -1
	}
	return -1
}

const maxLinkLabel = 999

func (r *renderer) ref(label string) (string, bool) {
	if len(r.refs) == 0 || len(label) > maxLinkLabel {
		return "", false
	}
	dest, ok := r.refs[normLabel(label)]
	return dest, ok
}

func linkDest(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "<") {
		if k := strings.IndexByte(s, '>'); k > 0 && !strings.Contains(s[:k], "\n") {
			return s[1:k]
		}
	}
	if k := strings.IndexAny(s, " \n"); k >= 0 {
		return s[:k]
	}
	return s
}

func normLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

type pairCache struct{ brackets, parens map[int]int }

func (r *renderer) link(s string, i int, pc *pairCache) (text, dest string, end int, ok bool) {
	j := match(s, i, '[', ']', true, pc.brackets)
	if j < 0 {
		return
	}
	text = s[i+1 : j]
	rest := s[j+1:]
	if strings.HasPrefix(rest, "(") {
		if k := match(s, j+1, '(', ')', false, pc.parens); k >= 0 {
			return text, linkDest(s[j+2 : k]), k + 1, true
		}
	} else if strings.HasPrefix(rest, "[") {
		if k := strings.IndexByte(rest, ']'); k > 0 {
			label := rest[1:k]
			if label == "" {
				label = text
			}
			dest, ok = r.ref(label)
			return text, dest, j + k + 2, ok
		}
	}
	dest, ok = r.ref(text)
	return text, dest, j + 1, ok
}

func entity(s string) (rune, int) {
	k := strings.IndexByte(s[:min(len(s), 10)], ';')
	if k < 2 {
		return 0, 0
	}
	name := s[1:k]
	if name[0] == '#' {
		base, digits := 10, name[1:]
		if strings.HasPrefix(digits, "x") || strings.HasPrefix(digits, "X") {
			base, digits = 16, digits[1:]
		}
		n, err := strconv.ParseUint(digits, base, 21)
		if err != nil || n < 0x20 || n >= 0x7f && n < 0xa0 {
			return 0, 0
		}
		return rune(n), k + 1
	}
	if r, ok := entities[name]; ok {
		return r, k + 1
	}
	return 0, 0
}

func htmlTag(inner string) (name string, ok bool) {
	t := strings.TrimPrefix(inner, "/")
	n := 0
	for n < len(t) && isAlnum(t[n]) {
		n++
	}
	if n == 0 || n < len(t) && t[n] != ' ' && t[n] != '/' && t[n] != '\n' {
		return "", false
	}
	name = strings.ToLower(t[:n])
	_, ok = htmlTags[name]
	return name, ok
}

func htmlAttr(tag, name string) string {
	k := strings.Index(tag, name+"=")
	if k < 0 || k+len(name)+1 >= len(tag) {
		return ""
	}
	v := tag[k+len(name)+1:]
	if q := v[0]; q == '"' || q == '\'' {
		if e := strings.IndexByte(v[1:], q); e >= 0 {
			return v[1 : 1+e]
		}
	}
	return ""
}

func isAutolink(s string) bool {
	if s == "" || strings.ContainsAny(s, " \n") {
		return false
	}
	for _, p := range [...]string{"http://", "https://", "mailto:", "ftp://"} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	at := strings.IndexByte(s, '@')
	return at > 0 && at < len(s)-1 && !strings.ContainsAny(s, "/:")
}

func (r *renderer) image(alt, src string, st style, link string) piece {
	alt = strings.ReplaceAll(alt, "\n", " ")
	text := "[image]"
	if alt != "" {
		text = "[image: " + alt + "]"
	}
	p := piece{text: text, st: st.with(style{attr: dim}), link: link}
	if r.gfx != nil && r.imgWidth > 0 {
		p.img = r.gfx.load(src, r.imgWidth)
	}
	return p
}

func (r *renderer) inline(s string, st style, link string, out []piece) []piece {
	// Once a search for a closing delimiter fails, every later search for the
	// same delimiter in s fails too; without this, unmatched openers make a
	// long paragraph quadratic.
	var dead [2][3][2]bool
	closer := func(from int, c byte, use int, exact bool) int {
		e := 0
		if exact {
			e = 1
		}
		d := &dead[c&1][use-1][e]
		if *d {
			return -1
		}
		k := findCloser(s, from, c, use, exact)
		*d = k < 0
		return k
	}
	var pc *pairCache
	pairs := func() *pairCache {
		if pc == nil {
			pc = &pairCache{map[int]int{}, map[int]int{}}
		}
		return pc
	}
	noComment := false
	var buf []byte
	flush := func() {
		if len(buf) > 0 {
			out = append(out, piece{text: string(buf), st: st, link: link})
			buf = buf[:0]
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch c {
		case '\\':
			if i+1 < len(s) && s[i+1] == '\n' {
				flush()
				out = append(out, piece{brk: true})
				i += 2
				continue
			}
			if i+1 < len(s) && isPunct(s[i+1]) {
				buf = append(buf, s[i+1])
				i += 2
				continue
			}
		case '\n':
			if n := len(buf); n >= 2 && buf[n-1] == ' ' && buf[n-2] == ' ' {
				flush()
				out = append(out, piece{brk: true})
			} else {
				buf = append(buf, ' ')
			}
			i++
			continue
		case '`':
			n := run(s, i, '`')
			if j := findRun(s, i+n, '`', n); j >= 0 {
				code := strings.ReplaceAll(s[i+n:j], "\n", " ")
				if len(code) > 2 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.Trim(code, " ") != "" {
					code = code[1 : len(code)-1]
				}
				if !r.color {
					code = "`" + code + "`"
				}
				flush()
				out = append(out, piece{text: code, st: st.with(codeStyle), link: link})
				i = j + n
				continue
			}
			buf = append(buf, s[i:i+n]...)
			i += n
			continue
		case '!':
			if i+1 < len(s) && s[i+1] == '[' {
				if alt, src, end, ok := r.link(s, i+1, pairs()); ok {
					flush()
					out = append(out, r.image(alt, src, st, link))
					i = end
					continue
				}
			}
		case '[':
			// Nested links would make deep nesting quadratic.
			if r.inLink {
				break
			}
			if text, dest, end, ok := r.link(s, i, pairs()); ok {
				flush()
				r.inLink = true
				if !r.color {
					out = r.inline(text, st, "", out)
					if dest != "" && dest != text {
						out = append(out, piece{text: " (" + dest + ")", st: st})
					}
				} else {
					out = r.inline(text, st.with(linkStyle), dest, out)
				}
				r.inLink = false
				i = end
				continue
			}
		case '<':
			if !noComment && strings.HasPrefix(s[i:], "<!--") {
				if j := strings.Index(s[i+4:], "-->"); j >= 0 {
					i += j + 7
					continue
				}
				noComment = true
			}
			// A tag ends before the next "<"; without that bound a run of
			// "<" rescans the rest of the text each time.
			if j := strings.IndexAny(s[i+1:], "<>") + 1; j > 0 && s[i+j] == '>' {
				inner := s[i+1 : i+j]
				if isAutolink(inner) {
					dest := inner
					if !strings.Contains(dest, ":") {
						dest = "mailto:" + dest
					}
					flush()
					out = append(out, piece{text: inner, st: st.with(linkStyle), link: dest})
					i += j + 1
					continue
				}
				if name, ok := htmlTag(inner); ok {
					flush()
					if name == "img" {
						out = append(out, r.image(htmlAttr(inner, "alt"), htmlAttr(inner, "src"), st, link))
					} else if htmlTags[name] {
						out = append(out, piece{brk: true})
					}
					i += j + 1
					continue
				}
			}
		case '&':
			if e, n := entity(s[i:]); n > 0 {
				buf = utf8.AppendRune(buf, e)
				i += n
				continue
			}
		case '*', '_':
			n := run(s, i, c)
			if open, _ := flank(s, i, n); open {
				use := min(n, 3)
				j := closer(i+n, c, use, true)
				// A run that closes in two steps (***a** b*) pairs its outer
				// delimiters with the later closer.
				for u := use - 1; u >= 1 && (j < 0 || use < min(n, 3)); u-- {
					if k := closer(i+n, c, u, true); k > j {
						use, j = u, k
					}
				}
				for u := min(n, 3); u >= 1 && j < 0; u-- {
					if k := closer(i+n, c, u, false); k >= 0 {
						use, j = u, k
					}
				}
				if j >= 0 {
					flush()
					out = r.inline(s[i+use:j], st.with(emph[use]), link, out)
					i = j + use
					continue
				}
			}
			buf = append(buf, s[i:i+n]...)
			i += n
			continue
		case '~':
			if strings.HasPrefix(s[i:], "~~") {
				if j := strings.Index(s[i+2:], "~~"); j > 0 {
					flush()
					out = r.inline(s[i+2:i+2+j], st.with(style{attr: strike}), link, out)
					i += j + 4
					continue
				}
			}
		case 'h':
			if (i == 0 || !isAlnum(s[i-1])) && (strings.HasPrefix(s[i:], "http://") || strings.HasPrefix(s[i:], "https://")) {
				j := i
				for j < len(s) && s[j] > ' ' && s[j] != '<' && s[j] != '`' {
					j++
				}
				url := strings.TrimRight(s[i:j], ".,;:!?'\"*_")
				if !strings.Contains(url, "(") {
					url = strings.TrimRight(url, ")")
				}
				dest := link
				if dest == "" {
					dest = url
				}
				flush()
				out = append(out, piece{text: url, st: st.with(linkStyle), link: dest})
				i += len(url)
				continue
			}
		}
		buf = append(buf, c)
		i++
	}
	flush()
	return out
}
