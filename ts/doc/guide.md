# How-to guide — railroad recipes

Focused recipes for real tasks. Each is self-contained. For the full API
see [reference.md](reference.md); for the why see [concepts.md](concepts.md).

All recipes assume:

```bash
npm install @tabnas/parser @tabnas/railroad @tabnas/json
```

## Diagram your own grammar

`railroad` draws whatever grammar is installed on the instance. Swap
`@tabnas/json` for your own grammar plugin:

```js ignore
const { Tabnas } = require('@tabnas/parser')
const { myGrammar } = require('my-grammar')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [myGrammar, railroad] })
console.log(tn.railroad.toAscii())
```

Install order is irrelevant — the helpers re-introspect the live grammar on
each call, so `[railroad, myGrammar]` works too.

## Save the three artifacts to disk

```js ignore
const fs = require('node:fs')
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })

fs.writeFileSync('grammar.railroad.json',
  JSON.stringify(tn.railroad.toJson(), null, 2))
fs.writeFileSync('grammar.svg', tn.railroad.toSvg())
fs.writeFileSync('grammar.txt', tn.railroad.toAscii())
```

Or let the CLI do it in one call (it writes exactly these three files):

```bash
npx tabnas-railroad --grammar @tabnas/json -o diagrams
```

## Get plain ASCII (no Unicode box-drawing)

For terminals or pipelines that choke on `│ ─ ┼`, pass `{ ascii: true }`
(CLI: `--ascii-plain`). Output is pure 7-bit ASCII using `| - +`.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const plain = tn.railroad.toAscii({ ascii: true })

/^[\x00-\x7F]*$/.test(plain)   // => true
```

## Get a compact text (EBNF-ish) summary

When you want one line per rule rather than a diagram, use `toText` on each
rule node. Terminals are quoted, choice is `(a | b)`, optional is `[x]`,
zero-or-more is `{x}`, one-or-more is `x+`.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad, toText } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const { rules } = tn.railroad.toJson()

toText(rules.map)    // => '"{" [pair] "}"'
toText(rules.val)    // => '(map | list | "VAL")'
toText(rules.pair)   // => '"KEY" ":" val+ /* "," */'
```

The CLI exposes the same as `--text`:

```bash
npx tabnas-railroad --grammar @tabnas/json --text -o /tmp/rr
```

## Save the model, render it later (decoupling)

The JSON model is the interchange format: SVG and ASCII are fully
reproducible from it, with no live instance needed.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad, modelToSvg, modelToAscii } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const model = tn.railroad.toJson()

// round-trip through JSON, then render — identical output.
const reloaded = JSON.parse(JSON.stringify(model))
modelToSvg(reloaded) === modelToSvg(model)       // => true
modelToAscii(reloaded) === modelToAscii(model)   // => true
```

With the CLI, write the model once and re-render any time:

```bash
npx tabnas-railroad --grammar @tabnas/json --json -o diagrams
npx tabnas-railroad -f diagrams/grammar.railroad.json --ascii
npx tabnas-railroad -f diagrams/grammar.railroad.json --svg > g.svg
cat diagrams/grammar.railroad.json | npx tabnas-railroad - --text
```

## Hand-build a diagram (no grammar at all)

The node constructors are exported, so you can draw any railroad diagram by
hand — useful for documentation snippets unrelated to a tabnas grammar.

```js
const { Sequence, Optional, toText } = require('@tabnas/railroad')

// bare strings are taken as terminals
toText(Sequence('GET', Optional('path')))   // => '"GET" ["path"]'
```

Render a single hand-built node to its own standalone SVG or ASCII block
with `renderNodeSvg` / `renderNodeAscii`:

```js
const { Sequence, Terminal, NonTerminal, Diagram, renderNodeSvg } =
  require('@tabnas/railroad')

const node = Diagram(Sequence(Terminal('GET'), NonTerminal('path')))
renderNodeSvg(node).startsWith('<svg ')   // => true
```

The constructors are also reachable through the plugin member as short
aliases (`tn.railroad.seq`, `.choice`, `.opt`, `.plus`, `.star`, `.t`,
`.n`, ...) — see [reference.md](reference.md).

## Read the token legend

Token labels that are not self-explanatory punctuation (e.g. the `KEY` /
`VAL` token sets) get a `legend` entry; tokens the lexer silently skips
(whitespace, comments) are reported in `ignored`. Both are also rendered
into the SVG and ASCII as "Tokens" / "Ignored tokens" keys.

```js
const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad } = require('@tabnas/railroad')

const tn = new Tabnas({ plugins: [json, railroad] })
const model = tn.railroad.toJson()

const tokens = Object.fromEntries(model.legend.map((e) => [e.token, e.meaning]))
'KEY' in tokens                          // => true
model.ignored.map((e) => e.token).sort() // => ['CM', 'LN', 'SP']
```

## Skip the normalization pass

By default extraction factors common prefix/suffix across choice branches
and turns an empty branch into `optional`. Pass `{ factor: false }` to get
the raw reverse-mapped tree (closer to the engine's alt structure):

```js ignore
const raw = tn.railroad.toJson({ factor: false })
```
