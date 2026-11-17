package main

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

type style struct{ attr, fg uint8 }

const (
	bold uint8 = 1 << iota
	dim
	italic
	underline
	strike
)

const (
	red uint8 = 31 + iota
	green
	yellow
	blue
	magenta
	cyan
)

var attrCodes = [...]byte{'1', '2', '3', '4', '9'}

func (s style) with(o style) style {
	s.attr |= o.attr
	if o.fg != 0 {
		s.fg = o.fg
	}
	return s
}

func (s style) writeSGR(b *strings.Builder) {
	b.WriteString("\x1b[")
	sep := false
	for i, c := range attrCodes {
		if s.attr&(1<<i) != 0 {
			if sep {
				b.WriteByte(';')
			}
			b.WriteByte(c)
			sep = true
		}
	}
	if s.fg != 0 {
		if sep {
			b.WriteByte(';')
		}
		b.WriteByte('0' + s.fg/10)
		b.WriteByte('0' + s.fg%10)
	}
	b.WriteByte('m')
}

type piece struct {
	text string
	st   style
	link string
	brk  bool
	img  *picture
}

type line struct {
	ps []piece
	w  int
}

var wide = &unicode.RangeTable{
	R16: []unicode.Range16{
		{0x1100, 0x115F, 1}, {0x231A, 0x231B, 1}, {0x23E9, 0x23EC, 1}, {0x23F0, 0x23F0, 1},
		{0x23F3, 0x23F3, 1}, {0x25FD, 0x25FE, 1}, {0x2614, 0x2615, 1}, {0x2648, 0x2653, 1},
		{0x267F, 0x267F, 1}, {0x2693, 0x2693, 1}, {0x26A1, 0x26A1, 1}, {0x26AA, 0x26AB, 1},
		{0x26BD, 0x26BE, 1}, {0x26C4, 0x26C5, 1}, {0x26CE, 0x26CE, 1}, {0x26D4, 0x26D4, 1},
		{0x26EA, 0x26EA, 1}, {0x26F2, 0x26F3, 1}, {0x26F5, 0x26F5, 1}, {0x26FA, 0x26FA, 1},
		{0x26FD, 0x26FD, 1}, {0x2705, 0x2705, 1}, {0x270A, 0x270B, 1}, {0x2728, 0x2728, 1},
		{0x274C, 0x274C, 1}, {0x274E, 0x274E, 1}, {0x2753, 0x2755, 1}, {0x2757, 0x2757, 1},
		{0x2795, 0x2797, 1}, {0x27B0, 0x27B0, 1}, {0x27BF, 0x27BF, 1}, {0x2B1B, 0x2B1C, 1},
		{0x2B50, 0x2B50, 1}, {0x2B55, 0x2B55, 1}, {0x2E80, 0x303E, 1}, {0x3041, 0x33FF, 1},
		{0x3400, 0x4DBF, 1}, {0x4E00, 0xA4CF, 1}, {0xA960, 0xA97F, 1}, {0xAC00, 0xD7A3, 1},
		{0xF900, 0xFAFF, 1}, {0xFE30, 0xFE4F, 1}, {0xFF00, 0xFF60, 1}, {0xFFE0, 0xFFE6, 1},
	},
	R32: []unicode.Range32{
		{0x1F004, 0x1F004, 1}, {0x1F0CF, 0x1F0CF, 1}, {0x1F18E, 0x1F18E, 1}, {0x1F191, 0x1F19A, 1},
		{0x1F200, 0x1F2FF, 1}, {0x1F300, 0x1F64F, 1}, {0x1F680, 0x1F6FF, 1}, {0x1F900, 0x1F9FF, 1},
		{0x1FA70, 0x1FAFF, 1}, {0x20000, 0x3FFFD, 1},
	},
}

func runeWidth(r rune) int {
	switch {
	case r < 0x20 || r == 0x7f:
		return 0
	case r < 0x300:
		return 1
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case unicode.Is(wide, r):
		return 2
	}
	return 1
}

func strWidth(s string) int {
	_, w := cutWidth(s, math.MaxInt)
	return w
}

func cutWidth(s string, max int) (n, w int) {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		rw := runeWidth(r)
		// VS16 turns a narrow symbol into a two-cell emoji. Counting it on the
		// base rune keeps the pair together when a word is split.
		if rw == 1 && strings.HasPrefix(s[i+size:], "\uFE0F") {
			rw = 2
		}
		if w+rw > max {
			return i, w
		}
		w += rw
		i += size
	}
	return len(s), w
}

func wrap(ps []piece, width int) []line {
	if width < 1 {
		width = 1
	}
	var (
		lines []line
		cur   line
		word  []piece
		wordW int
		space piece
	)
	push := func() {
		if len(cur.ps) > 0 {
			lines = append(lines, cur)
			cur = line{}
		}
	}
	flush := func() {
		if wordW == 0 {
			word = word[:0]
			return
		}
		if cur.w > 0 {
			if cur.w+1+wordW > width {
				push()
			} else {
				cur.ps = append(cur.ps, space)
				cur.w++
			}
		}
		for wordW > width {
			var head line
			for head.w < width && len(word) > 0 {
				p := &word[0]
				cut, cw := cutWidth(p.text, width-head.w)
				if cut == 0 {
					if head.w > 0 {
						break
					}
					_, cut = utf8.DecodeRuneInString(p.text)
					cw = strWidth(p.text[:cut])
				}
				h := *p
				h.text = p.text[:cut]
				head.ps = append(head.ps, h)
				head.w += cw
				if cut == len(p.text) {
					word = word[1:]
				} else {
					p.text = p.text[cut:]
				}
			}
			if head.w == 0 {
				break
			}
			lines = append(lines, head)
			wordW = 0
			for _, p := range word {
				wordW += strWidth(p.text)
			}
		}
		cur.ps = append(cur.ps, word...)
		cur.w += wordW
		word, wordW = word[:0], 0
	}
	for _, p := range ps {
		if p.brk {
			flush()
			push()
			continue
		}
		if p.img != nil {
			flush()
			push()
			lines = append(lines, line{ps: []piece{p}, w: p.img.cols})
			continue
		}
		t := p.text
		for len(t) > 0 {
			k := strings.IndexByte(t, ' ')
			if k != 0 {
				seg := t
				if k > 0 {
					seg = t[:k]
				}
				word = append(word, piece{text: seg, st: p.st, link: p.link})
				wordW += strWidth(seg)
				if k < 0 {
					break
				}
			}
			flush()
			space = piece{text: " ", st: p.st, link: p.link}
			t = t[k+1:]
		}
	}
	flush()
	push()
	return lines
}
