# mdv

A small, fast markdown viewer for the terminal, with **bold**, *italic*,
~~strikethrough~~, `inline code` and [links](https://github.com/mwing/mdv).

![Dusk over low hills](dusk.png)

## Lists and tasks

- Colours follow the terminal theme
  - Nested items get their own bullet
- [x] Tables, quotes and code blocks
- [ ] Syntax highlighting

> [!TIP]
> Long output is paged through `less`, so the mouse wheel scrolls.

## Code

```sh
mdv README.md
git show HEAD:README.md | mdv
```

## Table

| Flag      | Meaning                       | Default |
|-----------|-------------------------------|--------:|
| `-w N`    | Wrap at N columns             |  `auto` |
| `-images` | `auto`, `always` or `never`   |  `auto` |
| `-P`      | Do not page                   |     off |
