# @tabnas/railroad

Railroad (syntax) diagram renderer for the
[tabnas](https://github.com/rjrodger/tabnas) parser — introspects a tabnas
instance's installed grammar and emits a declarative JSON model, a
vertical-flow SVG, and a vertical ASCII diagram. Also ships a
`tabnas-railroad` CLI.

This repository contains:

| Path | Description |
|---|---|
| [`ts/`](ts/) | TypeScript / JavaScript implementation (`@tabnas/railroad`), plus the `tabnas-railroad` CLI. |

See [`ts/README.md`](ts/README.md) for usage.

## Sample output

The `@tabnas/json` grammar rendered to a vertical-flow diagram
(`tabnas-railroad --grammar @tabnas/json`):

![railroad diagram of the @tabnas/json grammar](examples/json-grammar.svg)

The same grammar as an ASCII diagram (excerpt — the `val` choice and the
`pair` loop; full output in
[`examples/json-grammar.txt`](examples/json-grammar.txt)):

```text
val:
              │
   ┌──────────┼──────────┐
┌──┴──┐   ┌───┴──┐   ╭───┴───╮
│ map │   │ list │   │ "VAL" │
└──┬──┘   └───┬──┘   ╰───┬───╯
   └──────────┼──────────┘
              │

pair:
    │
    ├────┐
╭───┴───╮│
│ "KEY" ││
╰───┬───╯│
    │    │
 ╭──┴──╮ │
 │ ":" │ │,
 ╰──┬──╯ │
    │    │
 ┌──┴──┐ │
 │ val │ │
 └──┬──┘ │
    ├────┘
    │
```

## License

MIT. Copyright (c) Richard Rodger.
