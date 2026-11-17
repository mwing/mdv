package main

import (
	"strconv"
	"strings"
)

type glyphs struct {
	bullets     [3]string
	todo, done  string
	quote, code string
	h1, rule    string
	tv, tx      string
}

var (
	unicodeGlyphs = glyphs{[3]string{"•", "◦", "▪"}, "☐", "☑", "┃", "│", "═", "─", "│", "┼"}
	asciiGlyphs   = glyphs{[3]string{"*", "-", "+"}, "[ ]", "[x]", ">", "|", "=", "-", "|", "+"}
)

var headingStyles = [...]style{{bold, magenta}, {bold, blue}, {bold, cyan}, {attr: bold}}

var alerts = map[string]struct {
	title string
	fg    uint8
}{
	"[!NOTE]":      {"Note", blue},
	"[!TIP]":       {"Tip", green},
	"[!IMPORTANT]": {"Important", magenta},
	"[!WARNING]":   {"Warning", yellow},
	"[!CAUTION]":   {"Caution", red},
}

type renderer struct {
	color bool
	g     *glyphs
	refs  map[string]string
	depth int

	gfx      *gfx
	imgWidth int
}

func (r *renderer) paint(st style, text string) string {
	if !r.color || st == (style{}) {
		return text
	}
	var b strings.Builder
	st.writeSGR(&b)
	b.WriteString(text)
	b.WriteString("\x1b[0m")
	return b.String()
}

func (r *renderer) emit(ps []piece) string {
	var b strings.Builder
	if !r.color {
		for _, p := range ps {
			b.WriteString(p.text)
		}
		return b.String()
	}
	var st style
	var link string
	end := func() {
		if st != (style{}) {
			b.WriteString("\x1b[0m")
		}
		if link != "" {
			b.WriteString("\x1b]8;;\x1b\\")
		}
	}
	for _, p := range ps {
		if p.st != st || p.link != link {
			end()
			st, link = p.st, p.link
			if link != "" {
				b.WriteString("\x1b]8;;" + link + "\x1b\\")
			}
			if st != (style{}) {
				st.writeSGR(&b)
			}
		}
		b.WriteString(p.text)
	}
	end()
	return b.String()
}

func blank(l string) bool { return strings.TrimLeft(l, " ") == "" }

func indent(l string) int {
	n := 0
	for n < len(l) && l[n] == ' ' {
		n++
	}
	return n
}

func cut(l string, n int) string {
	if n >= len(l) {
		return ""
	}
	return l[n:]
}

func fence(t string) (c byte, n int, ok bool) {
	if len(t) < 3 || t[0] != '`' && t[0] != '~' {
		return
	}
	n = run(t, 0, t[0])
	if n < 3 || t[0] == '`' && strings.IndexByte(t[n:], '`') >= 0 {
		return
	}
	return t[0], n, true
}

func atx(t string) (level int, text string) {
	n := run(t, 0, '#')
	if n == 0 || n > 6 || n < len(t) && t[n] != ' ' {
		return 0, ""
	}
	text = strings.TrimSpace(t[n:])
	if k := strings.TrimRight(text, "#"); k == "" || strings.HasSuffix(k, " ") {
		text = strings.TrimSpace(k)
	}
	return n, text
}

func isHR(t string) bool {
	if t == "" || !strings.ContainsRune("-*_", rune(t[0])) {
		return false
	}
	n := 0
	for i := 0; i < len(t); i++ {
		switch t[i] {
		case t[0]:
			n++
		case ' ':
		default:
			return false
		}
	}
	return n >= 3
}

func setext(l string) int {
	if indent(l) >= 4 {
		return 0
	}
	t := strings.TrimSpace(l)
	switch {
	case t == "":
		return 0
	case strings.Trim(t, "=") == "":
		return 1
	case strings.Trim(t, "-") == "":
		return 2
	}
	return 0
}

type marker struct {
	cind, num      int
	ordered, empty bool
}

func listMarker(l string) (m marker, ok bool) {
	ind := indent(l)
	t := l[ind:]
	n := 0
	switch {
	case t == "":
		return m, false
	case t[0] == '-' || t[0] == '+' || t[0] == '*':
		n = 1
	default:
		for n < len(t) && n < 9 && t[n] >= '0' && t[n] <= '9' {
			m.num = m.num*10 + int(t[n]-'0')
			n++
		}
		if n == 0 || n == len(t) || t[n] != '.' && t[n] != ')' {
			return m, false
		}
		n++
		m.ordered = true
	}
	sp := indent(t[n:])
	switch {
	case n+sp == len(t):
		m.empty = true
		sp = 1
	case sp == 0:
		return m, false
	case sp > 4:
		sp = 1
	}
	m.cind = ind + n + sp
	return m, true
}

// l must not be blank.
func interrupts(l string) bool {
	ind := indent(l)
	if ind >= 4 {
		return false
	}
	t := l[ind:]
	if t[0] == '>' || isHR(t) {
		return true
	}
	if lv, _ := atx(t); lv > 0 {
		return true
	}
	if _, _, ok := fence(t); ok {
		return true
	}
	m, ok := listMarker(l)
	return ok && !m.empty && (!m.ordered || m.num == 1)
}

func refDef(l string) (label, dest string, ok bool) {
	t := strings.TrimLeft(l, " ")
	if len(l)-len(t) > 3 || !strings.HasPrefix(t, "[") || strings.HasPrefix(t, "[^") {
		return
	}
	k := strings.Index(t, "]:")
	if k < 2 || strings.ContainsAny(t[1:k], "[]") {
		return
	}
	dest = linkDest(t[k+2:])
	return normLabel(t[1:k]), dest, dest != ""
}

func isDelimRow(l string) bool {
	t := strings.TrimSpace(l)
	return strings.Contains(t, "|") && strings.Contains(t, "-") && strings.Trim(t, "|:- ") == ""
}

func splitRow(l string) []string {
	l = strings.TrimPrefix(strings.TrimSpace(l), "|")
	if strings.HasSuffix(l, "|") && !strings.HasSuffix(l, "\\|") {
		l = l[:len(l)-1]
	}
	var cells []string
	start := 0
	for i := 0; i < len(l); i++ {
		switch l[i] {
		case '\\':
			i++
		case '|':
			cells = append(cells, strings.TrimSpace(l[start:i]))
			start = i + 1
		}
	}
	return append(cells, strings.TrimSpace(l[start:]))
}

func expandTabs(l string) string {
	if !strings.Contains(l, "\t") {
		return l
	}
	var b strings.Builder
	col := 0
	for i := 0; i < len(l); i++ {
		if l[i] == '\t' {
			n := 4 - col%4
			b.WriteString("    "[:n])
			col += n
		} else {
			b.WriteByte(l[i])
			col++
		}
	}
	return b.String()
}

// Front matter is only recognised when it opens with a key, so a document
// that starts with a rule is not swallowed up to its next rule.
func yamlKey(l string) bool {
	k := strings.IndexByte(l, ':')
	return k > 0 && !strings.ContainsAny(l[:k], " #*[>")
}

func gutter(bar string, ls []string) []string {
	for i, l := range ls {
		if l == "" {
			ls[i] = bar
		} else {
			ls[i] = bar + " " + l
		}
	}
	return ls
}

func (r *renderer) document(src string, width int) []string {
	ls := strings.Split(src, "\n")
	r.refs = map[string]string{}
	var open byte
	for i, l := range ls {
		l = expandTabs(l)
		ls[i] = l
		if c, _, ok := fence(strings.TrimLeft(l, " ")); ok {
			if open == 0 {
				open = c
			} else if open == c {
				open = 0
			}
		} else if open == 0 {
			if label, dest, ok := refDef(l); ok {
				if _, dup := r.refs[label]; !dup {
					r.refs[label] = dest
				}
			}
		}
	}
	var out []string
	if len(ls) > 2 && ls[0] == "---" && yamlKey(ls[1]) {
		for j := 2; j < len(ls); j++ {
			if ls[j] == "---" || ls[j] == "..." {
				for _, l := range ls[1:j] {
					out = append(out, r.paint(style{attr: dim}, l))
				}
				out = append(out, "")
				ls = ls[j+1:]
				break
			}
		}
	}
	out = append(out, r.render(ls, width, false)...)
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func (r *renderer) render(ls []string, width int, tight bool) []string {
	if width < 20 {
		width = 20
	}
	var out []string
	add := func(b []string) {
		if len(b) == 0 {
			return
		}
		if len(out) > 0 && !tight {
			out = append(out, "")
		}
		out = append(out, b...)
	}
	for i := 0; i < len(ls); {
		l := ls[i]
		if blank(l) {
			i++
			continue
		}
		ind := indent(l)
		t := l[ind:]

		if ind >= 4 {
			j := i
			for j < len(ls) && (blank(ls[j]) || indent(ls[j]) >= 4) {
				j++
			}
			for blank(ls[j-1]) {
				j--
			}
			code := make([]string, j-i)
			for k := range code {
				code[k] = cut(ls[i+k], 4)
			}
			add(r.code(code))
			i = j
			continue
		}

		if c, n, ok := fence(t); ok {
			var code []string
			i++
			for ; i < len(ls); i++ {
				ct := strings.TrimLeft(ls[i], " ")
				if c2, n2, ok := fence(ct); ok && c2 == c && n2 >= n && len(ls[i])-len(ct) < 4 && blank(ct[n2:]) {
					i++
					break
				}
				code = append(code, cut(ls[i], min(ind, indent(ls[i]))))
			}
			add(r.code(code))
			continue
		}

		if lv, text := atx(t); lv > 0 {
			add(r.heading(lv, text, width))
			i++
			continue
		}

		if isHR(t) {
			add([]string{r.paint(style{attr: dim}, strings.Repeat(r.g.rule, width))})
			i++
			continue
		}

		if t[0] == '>' {
			var inner []string
			for ; i < len(ls) && !blank(ls[i]); i++ {
				q := strings.TrimLeft(ls[i], " ")
				if q[0] == '>' {
					inner = append(inner, strings.TrimPrefix(q[1:], " "))
				} else if len(inner) > 0 && !blank(inner[len(inner)-1]) && !interrupts(ls[i]) {
					inner = append(inner, q)
				} else {
					break
				}
			}
			add(r.quote(inner, width))
			continue
		}

		if _, ok := listMarker(l); ok {
			var b []string
			b, i = r.list(ls, i, width)
			add(b)
			continue
		}

		if strings.HasPrefix(t, "<!--") && !strings.Contains(t, "-->") {
			for ; i < len(ls); i++ {
				if strings.Contains(ls[i], "-->") {
					i++
					break
				}
			}
			continue
		}

		if _, _, ok := refDef(l); ok {
			i++
			continue
		}

		if strings.Contains(l, "|") && i+1 < len(ls) && isDelimRow(ls[i+1]) {
			rows := [][]string{splitRow(l)}
			delim := splitRow(ls[i+1])
			for i += 2; i < len(ls) && !blank(ls[i]) && strings.Contains(ls[i], "|"); i++ {
				rows = append(rows, splitRow(ls[i]))
			}
			add(r.table(rows, delim, width))
			continue
		}

		j, level := i+1, 0
		for j < len(ls) && !blank(ls[j]) {
			if level = setext(ls[j]); level > 0 || interrupts(ls[j]) {
				break
			}
			j++
		}
		para := make([]string, j-i)
		for k := range para {
			para[k] = strings.TrimLeft(ls[i+k], " ")
		}
		text := strings.TrimRight(strings.Join(para, "\n"), " ")
		if level > 0 {
			add(r.heading(level, text, width))
			j++
		} else {
			add(r.para(text, style{}, width))
		}
		i = j
	}
	return out
}

func (r *renderer) para(text string, st style, width int) []string {
	r.imgWidth = width
	ps := r.inline(text, st, "", nil)
	r.imgWidth = 0
	ls := wrap(ps, width)
	out := make([]string, 0, len(ls))
	for _, l := range ls {
		if img := l.ps[0].img; img != nil {
			out = append(out, img.lines()...)
		} else {
			out = append(out, r.emit(l.ps))
		}
	}
	return out
}

func (r *renderer) heading(level int, text string, width int) []string {
	st := headingStyles[min(level, len(headingStyles))-1]
	var ps []piece
	if !r.color && level > 2 {
		ps = append(ps, piece{text: strings.Repeat("#", level) + " "})
	}
	ls := wrap(r.inline(text, st, "", ps), width)
	out := make([]string, 0, len(ls)+1)
	w := 0
	for _, l := range ls {
		out = append(out, r.emit(l.ps))
		w = max(w, l.w)
	}
	if level <= 2 && w > 0 {
		g := r.g.h1
		if level == 2 {
			g = r.g.rule
		}
		out = append(out, r.paint(style{fg: st.fg}, strings.Repeat(g, w)))
	}
	return out
}

// Code lines are never hard-wrapped, so a copied line stays one line.
func (r *renderer) code(ls []string) []string {
	return gutter(r.paint(style{attr: dim}, r.g.code), ls)
}

func (r *renderer) quote(inner []string, width int) []string {
	st := style{attr: dim}
	var out []string
	if len(inner) > 0 {
		if a, ok := alerts[strings.ToUpper(strings.TrimSpace(inner[0]))]; ok {
			st = style{fg: a.fg}
			out = append(out, r.paint(style{bold, a.fg}, a.title))
			inner = inner[1:]
		}
	}
	out = append(out, r.render(inner, width-2, false)...)
	return gutter(r.paint(st, r.g.quote), out)
}

func (r *renderer) list(ls []string, i, width int) ([]string, int) {
	first, _ := listMarker(ls[i])
	var items [][]string
	loose := false
	for i < len(ls) {
		m, ok := listMarker(ls[i])
		if !ok || m.ordered != first.ordered || indent(ls[i]) >= 4 || isHR(strings.TrimSpace(ls[i])) {
			break
		}
		it := []string{cut(ls[i], m.cind)}
		i++
	body:
		for i < len(ls) {
			l := ls[i]
			switch {
			case blank(l):
				k := i
				for k < len(ls) && blank(ls[k]) {
					k++
				}
				if k < len(ls) && indent(ls[k]) >= m.cind {
					for ; i < k; i++ {
						it = append(it, "")
					}
					loose = true
					continue
				}
				if k < len(ls) && indent(ls[k]) < 4 {
					if m2, ok := listMarker(ls[k]); ok && m2.ordered == first.ordered && !isHR(strings.TrimSpace(ls[k])) {
						loose = true
						i = k
					}
				}
				break body
			case indent(l) >= m.cind:
				it = append(it, l[m.cind:])
			case interrupts(l):
				break body
			default:
				if _, ok := listMarker(l); ok {
					break body
				}
				it = append(it, strings.TrimLeft(l, " "))
			}
			i++
		}
		items = append(items, it)
	}

	r.depth++
	defer func() { r.depth-- }()
	numW := len(strconv.Itoa(first.num + len(items) - 1))
	var out []string
	for n, it := range items {
		var mark string
		st := style{fg: cyan}
		switch {
		case first.ordered:
			mark = strconv.Itoa(first.num+n) + "."
			mark = strings.Repeat(" ", numW+1-len(mark)) + mark
		case strings.HasPrefix(it[0], "[ ] "):
			mark, it[0] = r.g.todo, it[0][4:]
		case strings.HasPrefix(it[0], "[x] "), strings.HasPrefix(it[0], "[X] "):
			mark, st, it[0] = r.g.done, style{fg: green}, it[0][4:]
		default:
			mark = r.g.bullets[(r.depth-1)%3]
		}
		pw := strWidth(mark) + 1
		body := r.render(it, width-pw, !loose)
		if n > 0 && loose {
			out = append(out, "")
		}
		if len(body) == 0 {
			body = []string{""}
		}
		pad := strings.Repeat(" ", pw)
		for k, l := range body {
			switch {
			case k == 0:
				out = append(out, strings.TrimRight(r.paint(st, mark)+" "+l, " "))
			case l == "":
				out = append(out, "")
			default:
				out = append(out, pad+l)
			}
		}
	}
	return out, i
}

func (r *renderer) table(rows [][]string, delim []string, width int) []string {
	ncol := len(rows[0])
	cells := make([][][]piece, len(rows))
	widths := make([]int, ncol)
	for y, row := range rows {
		cells[y] = make([][]piece, ncol)
		st := style{}
		if y == 0 {
			st.attr = bold
		}
		for x := 0; x < ncol && x < len(row); x++ {
			ps := r.inline(row[x], st, "", nil)
			cells[y][x] = ps
			for _, l := range wrap(ps, 1<<30) {
				widths[x] = max(widths[x], l.w)
			}
		}
	}

	avail := width - 3*(ncol-1) - 2
	fits := func(limit int) bool {
		sum := 0
		for _, w := range widths {
			sum += min(w, limit)
		}
		return sum <= avail
	}
	lo, hi := 1, 1
	for _, w := range widths {
		hi = max(hi, w)
	}
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fits(mid) {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	for x := range widths {
		widths[x] = max(1, min(widths[x], lo))
	}

	sep := " " + r.paint(style{attr: dim}, r.g.tv) + " "
	var rule strings.Builder
	for x, w := range widths {
		if x > 0 {
			rule.WriteString(r.g.tx)
		}
		rule.WriteString(strings.Repeat(r.g.rule, w+2))
	}
	ruleLine := r.paint(style{attr: dim}, rule.String())

	var out []string
	laid := make([][][]line, len(rows))
	wrapped := false
	for y := range cells {
		laid[y] = make([][]line, ncol)
		for x := range cells[y] {
			laid[y][x] = wrap(cells[y][x], widths[x])
			wrapped = wrapped || len(laid[y][x]) > 1
		}
	}
	for y, row := range laid {
		if y == 1 || y > 1 && wrapped {
			out = append(out, ruleLine)
		}
		h := 1
		for _, c := range row {
			h = max(h, len(c))
		}
		for k := 0; k < h; k++ {
			var b strings.Builder
			b.WriteByte(' ')
			for x, c := range row {
				if x > 0 {
					b.WriteString(sep)
				}
				text, w := "", 0
				if k < len(c) {
					text, w = r.emit(c[k].ps), c[k].w
				}
				// A two-cell rune overflows a column squeezed to one cell.
				gap := max(0, widths[x]-w)
				left := 0
				if x < len(delim) && strings.HasSuffix(delim[x], ":") {
					left = gap
					if strings.HasPrefix(delim[x], ":") {
						left = gap / 2
					}
				}
				b.WriteString(strings.Repeat(" ", left))
				b.WriteString(text)
				b.WriteString(strings.Repeat(" ", gap-left))
			}
			out = append(out, strings.TrimRight(b.String(), " "))
		}
	}
	return out
}
