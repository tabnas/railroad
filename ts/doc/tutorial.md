# Tutorial — your first railroad diagram

This walks you from nothing to a rendered syntax (railroad) diagram of a
real grammar. One happy path, every step verified. By the end you will have
a JSON model, an SVG, and an ASCII diagram of the `@tabnas/json` grammar.

`@tabnas/railroad` never parses anything. It **introspects a live
[`tabnas`](https://github.com/rjrodger/tabnas) parser instance that already
has a grammar installed** and draws that grammar.

## 1. Install

```bash
npm install @tabnas/parser @tabnas/railroad @tabnas/json
```

- `@tabnas/parser` is the parsing engine (a peer dependency).
- `@tabnas/railroad` is this package.
- `@tabnas/json` is a grammar to draw — any tabnas grammar plugin works.

## 2. Build an instance with a grammar and the plugin

Load a grammar plugin (`json`) and `railroad` onto a fresh `Tabnas`. Order
does not matter — `railroad` re-reads the grammar each time you call it.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })

typeof tn.railroad   // => 'function'
```

Loading the plugin decorates the instance with `tn.railroad`: a callable
that returns the grammar model, plus render helpers hanging off it.

## 3. Read the grammar back out as a model

`tn.railroad.toJson()` introspects the installed rules and returns a plain,
JSON-serializable `GrammarModel`.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const model = tn.railroad.toJson()

model.start                       // => 'val'
Object.keys(model.rules).length   // => 5
model.meta.engine                 // => 'tabnas'
```

The model has a `start` rule (the entry point), a `rules` map (one diagram
node per rule), and `meta`. The five json rules are `val`, `map`, `list`,
`pair`, `elem`.

## 4. Render an ASCII diagram

`tn.railroad.toAscii()` returns a vertical-flow ASCII diagram — one block
per rule. It is the quickest way to *see* the grammar in a terminal.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const ascii = tn.railroad.toAscii()

ascii.includes('val:')    // => true
ascii.includes('pair:')   // => true
```

The `val` rule renders as a three-way choice, read top to bottom:

```text
val:
              │
   ┌──────────┼──────────┐
┌──┴──┐   ┌───┴──┐   ╭───┴───╮
│ map │   │ list │   │ "VAL" │
└──┬──┘   └───┬──┘   ╰───┬───╯
   └──────────┼──────────┘
              │
```

Square-cornered boxes are **nonterminals** (rule references — `map`,
`list`); round-cornered quoted boxes are **terminals** (tokens — `"VAL"`).
A json value is a map, a list, or a value token.

The `pair` rule shows a repetition: a `KEY : val` group that loops back on
itself, with `,` on the return rail.

```text
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

If your terminal mangles the box-drawing characters, pass plain mode:

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const plain = tn.railroad.toAscii({ ascii: true })

/^[\x00-\x7F]*$/.test(plain)   // => true
```

## 5. Render an SVG

`tn.railroad.toSvg()` returns a standalone SVG string: a vertical stack of
titled, anchored rule tracks where nonterminal boxes link to the referenced
rule. Save it and open it in a browser.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const svg = tn.railroad.toSvg()

svg.startsWith('<svg ')   // => true
svg.endsWith('</svg>')    // => true
```

```js ignore
const fs = require('node:fs')
fs.writeFileSync('json-grammar.svg', tn.railroad.toSvg())
```

## 6. Do it all from the command line

You do not need to write code. The CLI ships with the package:

```bash
# write grammar.railroad.json + grammar.svg + grammar.txt into ./diagrams
npx tabnas-railroad --grammar @tabnas/json -o diagrams
```

That introspects `@tabnas/json` on a fresh instance and writes all three
artifacts. You now have the same model, SVG, and ASCII you produced in code.

## Where to go next

- [guide.md](guide.md) — focused recipes (save artifacts, plain ASCII,
  hand-built diagrams, the CLI render mode).
- [reference.md](reference.md) — every export, option, and CLI flag.
- [concepts.md](concepts.md) — how grammar introspection and the
  vertical-flow layout actually work.
