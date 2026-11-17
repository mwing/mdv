---
title: Sample
---

# mdv sample document

A paragraph with **bold**, *italic*, ***both***, ~~struck~~, `inline code`, a
[link](https://example.com/path), a [reference link][ref], <https://auto.link>, a bare
https://bare.example.org/x, snake_case_word, 2 * 3 * 4 and an escaped \*star\*.
Entities: &copy; &amp; &#x2192;. Hard break here  
next line. **bold with *nested italic* inside** and ***a** b*.

Setext heading
--------------

### Third level with `code`

#### Fourth level

- First item
- Second item with a long line that should wrap nicely under the bullet with a hanging indent rather than at column zero
  - Nested item
    - Deeper item
  - Another nested
- [ ] Todo
- [x] Done

1. One
2. Two

   Loose paragraph inside two.

   ```sh
   echo "code in list"
   ```
3. Three

> A quote with **bold** text that is long enough to wrap around the edge of the terminal at least once, hopefully.
>
> - list in quote
> > nested quote

> [!WARNING]
> Careful with this.

```go
func main() {
	fmt.Println("hello")   // tab-indented

}
```

    indented code block

| Name | Align right | Centered | Description |
|------|------------:|:--------:|-------------|
| `a` | 1 | x | Short |
| **b** | 22 | yy | A much longer description cell that has to wrap when the terminal is not wide enough to hold all of it |
| 日本語 | 333 | ✅ | wide \| chars |

* * *

<p align="center"><img src="logo.png" alt="Logo"></p>

<details><summary>Summary title</summary>
Hidden content with Vec<T> kept.
</details>

<!-- a comment
spanning lines -->

Badge: [![Build](https://img.shields.io/b.svg)](https://ci.example.com) end.

[ref]: https://example.com/ref "Title"
