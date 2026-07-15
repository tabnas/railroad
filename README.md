# @tabnas/railroad

<!-- tabnas-badges -->
[![npm](https://tabnas.github.io/status/badges/railroad-npm.svg)](https://www.npmjs.com/package/@tabnas/railroad)
[![CI](https://github.com/tabnas/railroad/actions/workflows/ci.yml/badge.svg)](https://github.com/tabnas/railroad/actions/workflows/ci.yml)
[![go](https://tabnas.github.io/status/badges/railroad-go.svg)](https://pkg.go.dev/github.com/tabnas/railroad/go)
[![tabnas standard](https://tabnas.github.io/status/badges/railroad-standard.svg)](https://tabnas.github.io/status/)
<!-- /tabnas-badges -->

Railroad (syntax) diagram renderer for the
[tabnas](https://github.com/rjrodger/tabnas) parser — introspects a tabnas
instance's installed grammar and emits a declarative JSON model, a
vertical-flow SVG, and a vertical ASCII diagram. Also ships a
`tabnas-railroad` CLI.

This repository contains:

| Path | Description |
|---|---|
| [`ts/`](ts/) | TypeScript / JavaScript implementation (`@tabnas/railroad`), plus the `tabnas-railroad` CLI. **Canonical.** |
| [`go/`](go/) | Go port (`package tabnasrailroad`), plus the `cmd/tabnas-railroad` CLI. Tracks `ts/`. |

See [`ts/README.md`](ts/README.md) and [`go/README.md`](go/README.md) for usage.

## Documentation

Four-quadrant [Diátaxis](https://diataxis.fr/) docs, per language:

| | TypeScript (canonical) | Go (port) |
|---|---|---|
| **Tutorial** (learn) | [ts/doc/tutorial.md](ts/doc/tutorial.md) | [go/doc/tutorial.md](go/doc/tutorial.md) |
| **How-to** (tasks) | [ts/doc/guide.md](ts/doc/guide.md) | [go/doc/guide.md](go/doc/guide.md) |
| **Reference** (API/CLI) | [ts/doc/reference.md](ts/doc/reference.md) | [go/doc/reference.md](go/doc/reference.md) |
| **Concepts** (why) | [ts/doc/concepts.md](ts/doc/concepts.md) | [go/doc/concepts.md](go/doc/concepts.md) |

## A tiny taste

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const model = tn.railroad.toJson()

model.start                       // => 'val'
Object.keys(model.rules).length   // => 5
```

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
