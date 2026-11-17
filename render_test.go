package main

import (
	"os"
	"strings"
	"testing"
	"time"
)

func plain(src string, width int) string {
	r := &renderer{g: &unicodeGlyphs}
	return strings.Join(r.document(src, width), "\n")
}

func TestRender(t *testing.T) {
	tests := []struct {
		name, src, want string
		width           int
	}{
		{"emphasis", "**b**, *i*, ***bi***, ~~s~~, `c`", "b, i, bi, s, `c`", 40},
		{"nested emphasis", "**a *b* c** and *a **b** c*", "a b c and a b c", 40},
		{"two-step closer", "***a** b* and ***a* b**", "a b and a b", 40},
		{"literal stars", "2 * 3 * 4, snake_case_word, \\*x\\*", "2 * 3 * 4, snake_case_word, *x*", 40},
		{"link", "[t](http://x.y/z \"title\")", "t (http://x.y/z)", 40},
		{"reference link", "[t][R] and [r]\n\n[r]: http://x.y", "t (http://x.y) and r (http://x.y)", 40},
		{"autolink", "<http://x.y> http://a.b/c.", "http://x.y http://a.b/c.", 40},
		{"image in link", "[![alt](i.png)](http://x.y)", "[image: alt] (http://x.y)", 40},
		{"html", "a<br>b <span>c</span> Vec<T> &amp; <!-- gone -->d", "a\nb c Vec<T> & d", 40},
		{"hard break", "a  \nb\\\nc\nd", "a\nb\nc d", 40},
		{"wrap", "aaa bbb ccc ddd eee fff ggg hhh", "aaa bbb ccc ddd eee\nfff ggg hhh", 20},
		{"headings", "# One\n\nTwo\n---\n### Three ###", "One\n═══\n\nTwo\n───\n\n### Three", 40},
		{"tight list", "- a\n- b\n  - c\n1. x\n2. y", "• a\n• b\n  ◦ c\n\n1. x\n2. y", 40},
		{"loose list", "9. a\n\n10. b", " 9. a\n\n10. b", 40},
		{"tasks", "- [ ] a\n- [x] b", "☐ a\n☑ b", 40},
		{"lazy list line", "- a\nb\n\nc", "• a b\n\nc", 40},
		{"fence", "```go\na\n\n  b\n```\nc", "│ a\n│\n│   b\n\nc", 40},
		{"unclosed fence", "~~~\n# a", "│ # a", 40},
		{"quote", "> a\n> > b\nc", "┃ a\n┃\n┃ ┃ b c", 40},
		{"alert", "> [!NOTE]\n> a", "┃ Note\n┃ a", 40},
		{"table", "| a | b |\n|:-:|--:|\n| ccc | \\| |\n| d |", "  a  │ b\n─────┼───\n ccc │ |\n  d  │", 40},
		{"front matter", "---\na: 1\n---\nx", "a: 1\n\nx", 40},
		{"rule is not a list", "a\n\n* * *\n\nb", "a\n\n────────────────────\n\nb", 20},
		{"rule then rule is not front matter", "---\n\n# T\n\n---", "────────────────────\n\nT\n═\n\n────────────────────", 20},
		{"text after comment", "<!-- c --> hello\n\n<!-- a\nb -->\nx", "hello\n\nx", 40},
		{"newline in destination", "> [a](<x\ny>) ![p\nq](z.png)", "┃ a (<x) [image: p q]", 40},
		{"wide rune in squeezed table", "|a|\n|-|\n|b<br>日本|", " a\n──────\n b\n 日本", 40},
		{"emoji ends a full-width word", "aaaaaaaaaaaaaaaaaaa✔️ b", "aaaaaaaaaaaaaaaaaaa\n✔️ b", 20},
		{"wide chars", "日本語日本語日本語日本 abc", "日本語日本語日本語日\n本 abc", 20},
	}
	for _, tt := range tests {
		if got := plain(tt.src, tt.width); got != tt.want {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
}

func TestColor(t *testing.T) {
	r := &renderer{g: &unicodeGlyphs, color: true}
	got := strings.Join(r.document("**b** [t](http://x.y) `c`", 40), "\n")
	want := "\x1b[1mb\x1b[0m \x1b]8;;http://x.y\x1b\\\x1b[4;34mt\x1b[0m\x1b]8;;\x1b\\ \x1b[33mc\x1b[0m"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestClean(t *testing.T) {
	if got := clean([]byte("\xef\xbb\xbfa\x1b[31mb\r\n\tc\u009b1md\u00a0é\x9b2J\xc2\x01\x9bx")); got != "a[31mb\n\tc1md\u00a0é\ufffd2J\ufffd\ufffdx" {
		t.Errorf("got %q", got)
	}
}

func TestWidthsStayInBounds(t *testing.T) {
	src, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []int{20, 40, 72, 100} {
		for i, l := range strings.Split(plain(clean(src), width), "\n") {
			if w := strWidth(l); w > width && !strings.Contains(l, "│") {
				t.Errorf("width %d: line %d is %d cells: %q", width, i+1, w, l)
			}
		}
	}
}

func TestLongWordStaysLinear(t *testing.T) {
	start := time.Now()
	if got := plain(strings.Repeat("a", 600000), 80); len(got) != 600000+7499 {
		t.Errorf("got %d bytes", len(got))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func TestCodeOverflowCountsScreenRows(t *testing.T) {
	r := &renderer{g: &unicodeGlyphs, cols: 10}
	r.document("```\n"+strings.Repeat("x", 25)+"\nshort\n```", 10)
	if r.overflow != 2 {
		t.Errorf("overflow = %d, want 2", r.overflow)
	}
}

func TestSqueezedTableDoesNotPanic(t *testing.T) {
	plain("|a|b|c|d|e|f|g|h|i|\n|-|-|-|-|-|-|-|-|-|\n|🚀|日本|c|d|e|f|g|h|i|", 20)
}

func TestUnmatchedDelimitersStayLinear(t *testing.T) {
	start := time.Now()
	plain(strings.Repeat("*a\n", 50000)+strings.Repeat("_b ", 50000), 80)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}

func BenchmarkRender(b *testing.B) {
	src, _ := os.ReadFile("testdata/sample.md")
	doc := strings.Repeat(clean(src)+"\n", 100)
	b.SetBytes(int64(len(doc)))
	r := &renderer{g: &unicodeGlyphs, color: true}
	for b.Loop() {
		r.document(doc, 100)
	}
}
