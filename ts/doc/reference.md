# Reference — `@tabnas/railroad`

The complete public surface: the plugin member, the bare exports, the
option and model types, and the `tabnas-railroad` CLI. Dry and complete.

Import from the package root:

```js ignore
const {
  railroad, extractGrammar, modelToSvg, modelToAscii,
  renderNodeSvg, renderNodeAscii, toText,
  Diagram, Sequence, Choice, Optional, OneOrMore, ZeroOrMore,
  Terminal, NonTerminal, Comment, Skip, RailroadError,
} = require('@tabnas/railroad')
```

## The plugin

```ts
const railroad: Plugin
```

Loading `railroad` (via `new Tabnas({ plugins: [railroad] })` or
`tn.use(railroad)`) decorates the instance with `tn.railroad`. Decoration
is **lazy**: each helper re-reads the instance's current grammar on call,
so install order relative to a grammar plugin does not matter. Child
instances created with `tn.make()` inherit the decoration.

### `tn.railroad` — the `RailroadApi`

`tn.railroad` is **callable** and also carries helpers and constructors.

| Member | Signature | Returns |
|---|---|---|
| `tn.railroad(opts?)` | `(opts?: ExtractOptions) => GrammarModel` | the grammar model |
| `tn.railroad.toJson(opts?)` | `(opts?: ExtractOptions) => GrammarModel` | alias of the call form |
| `tn.railroad.extract(opts?)` | `(opts?: ExtractOptions) => GrammarModel` | alias of the call form |
| `tn.railroad.toSvg(opts?)` | `(opts?: SvgOptions & ExtractOptions) => string` | whole-grammar SVG |
| `tn.railroad.toAscii(opts?)` | `(opts?: AsciiOptions & ExtractOptions) => string` | whole-grammar ASCII |
| `tn.railroad.renderNode(node, opts?)` | `(node: Item, opts?: SvgOptions) => string` | single-node SVG |
| `tn.railroad.renderNodeAscii(node, opts?)` | `(node: Item, opts?: AsciiOptions) => string` | single-node ASCII |
| `tn.railroad.renderNodeText(node)` | `(node: Item) => string` | single-node EBNF text |

The model constructors are also attached as short aliases:

| Alias | Constructor | Alias | Constructor |
|---|---|---|---|
| `tn.railroad.Diagram` | `Diagram` | `tn.railroad.t` | `Terminal` |
| `tn.railroad.seq` | `Sequence` | `tn.railroad.n` | `NonTerminal` |
| `tn.railroad.choice` | `Choice` | `tn.railroad.comment` | `Comment` |
| `tn.railroad.opt` | `Optional` | `tn.railroad.skip` | `Skip` |
| `tn.railroad.plus` | `OneOrMore` | | |
| `tn.railroad.star` | `ZeroOrMore` | | |

## Bare exports (instance-free)

These are exported directly from the package and need no live instance
(except `extractGrammar`, which takes one).

### `extractGrammar(tn, opts?) => GrammarModel`

Introspects a live `Tabnas` instance into a `GrammarModel`. This is what
`tn.railroad.toJson()` calls. `extractGrammar(tn)` and
`tn.railroad.toJson()` return deep-equal models for the same instance.

### `modelToSvg(model, opts?) => string`

Render a whole `GrammarModel` to a standalone SVG document: a vertical
stack of titled, anchored rule tracks (`<g id="rule">`), nonterminal boxes
linking to the referenced rule (`<a href="#rule">`), plus "Tokens" and
"Ignored tokens" key blocks when the model carries them. The diagram is
height-biased (taller than wide). `opts` is currently `SvgOptions` (its
internal `linkFor` is supplied automatically).

### `modelToAscii(model, opts?) => string`

Render a whole `GrammarModel` to a vertical ASCII diagram, one titled block
per rule, followed by "Tokens:" / "Ignored tokens:" key blocks when
present. `opts.ascii === true` selects plain `| - +` glyphs.

### `renderNodeSvg(node, opts?) => string`

Render a single node (a `RailroadNode` or a bare string terminal) to its
own standalone SVG document, with entry/exit rail caps.

### `renderNodeAscii(node, opts?) => string`

Render a single node to an ASCII block. `opts.ascii === true` for plain.

### `toText(node) => string`

Compact EBNF-ish rendering of a single node:

| Node | Text |
|---|---|
| `Terminal('x')` | `"x"` (JSON-quoted) |
| `NonTerminal('x')` | `x` |
| `Comment('x')` | `/* x */` |
| `Skip()` | `` (empty) |
| `Sequence(a, b)` | `a b` (empty parts dropped) |
| `Choice(a, b)` | `(a \| b)` |
| `Optional(a)` | `[a]` |
| `OneOrMore(a)` | `a+` |
| `OneOrMore(a, sep)` | `a+ /* sep */` |
| `ZeroOrMore(a)` | `{a}` |
| `ZeroOrMore(a, sep)` | `{a} /* sep */` |

```js
const { toText, Sequence, Choice, Optional, OneOrMore, ZeroOrMore } =
  require('@tabnas/railroad')

toText(Sequence('a', 'b'))     // => '"a" "b"'
toText(Choice('a', 'b'))       // => '("a" | "b")'
toText(Optional('a'))          // => '["a"]'
toText(OneOrMore('a'))         // => '"a"+'
toText(ZeroOrMore('a'))        // => '{"a"}'
```

## Node constructors

Each returns a plain `RailroadNode`. Arguments typed `Item` accept a
`RailroadNode` **or** a bare string (taken as a terminal).

| Constructor | Signature | Node |
|---|---|---|
| `Terminal(text)` | `(text: string)` | `{ kind: 'terminal', text }` |
| `NonTerminal(text)` | `(text: string)` | `{ kind: 'nonterminal', text }` |
| `Comment(text)` | `(text: string)` | `{ kind: 'comment', text }` |
| `Skip()` | `()` | `{ kind: 'skip' }` |
| `Sequence(...items)` | `(...items: Item[])` | `{ kind: 'seq', items }` |
| `Choice(...items)` | `(...items: Item[])` | `{ kind: 'choice', items }` |
| `Optional(item)` | `(item: Item)` | `{ kind: 'optional', item }` |
| `OneOrMore(item, rep?)` | `(item: Item, rep?: Item)` | `{ kind: 'oneOrMore', item, rep? }` |
| `ZeroOrMore(item, rep?)` | `(item: Item, rep?: Item)` | `{ kind: 'zeroOrMore', item, rep? }` |
| `Diagram(...items)` | `(...items: Item[])` | `{ kind: 'diagram', items }` |

`Choice` with **no** branches throws `RailroadError`. The `rep` of a
repetition is the node drawn on the return rail (typically a separator).

## Types

### `RailroadNode`

A tagged union (`kind` is the tag):

```ts
type RailroadNode =
  | { kind: 'terminal'; text: string }
  | { kind: 'nonterminal'; text: string }
  | { kind: 'comment'; text: string }
  | { kind: 'skip' }
  | { kind: 'seq'; items: RailroadNode[] }
  | { kind: 'choice'; items: RailroadNode[] }
  | { kind: 'optional'; item: RailroadNode }
  | { kind: 'oneOrMore'; item: RailroadNode; rep?: RailroadNode }
  | { kind: 'zeroOrMore'; item: RailroadNode; rep?: RailroadNode }
  | { kind: 'diagram'; items: RailroadNode[] }
```

`Item = RailroadNode | string` — a bare string is coerced to a `Terminal`
by the constructors and renderers.

### `GrammarModel`

```ts
type GrammarModel = {
  start: string                                    // entry rule name
  rules: { [name: string]: RailroadNode }          // one node per rule
  legend?: { token: string; meaning: string }[]    // named-token key
  ignored?: { token: string; meaning: string }[]   // lexer IGNORE set
  meta?: { engine: string; [k: string]: any }      // { engine: 'tabnas' }
}
```

Pure JSON-serializable data. `legend` / `ignored` are present only when
non-empty. The model is the interchange format — SVG/ASCII are reproducible
from it alone.

### `ExtractOptions`

```ts
type ExtractOptions = {
  factor?: boolean   // prefix/suffix factoring + empty->optional (default true)
  start?: string     // override the entry rule (default config.rule.start)
}
```

### `SvgOptions`

```ts
type SvgOptions = {
  linkFor?: (name: string) => string | undefined   // nonterminal -> href
}
```

`modelToSvg` supplies `linkFor` itself (linking nonterminals to their rule
tracks); you rarely set it.

### `AsciiOptions`

```ts
type AsciiOptions = {
  ascii?: boolean   // true => plain | - + glyphs instead of Unicode
}
```

### `RailroadError`

```ts
class RailroadError extends Error {
  name = 'RailroadError'
  node?: unknown   // the offending node, when applicable
}
```

Thrown for a malformed model: a `Choice` with no branches, an invalid node
value (e.g. `Sequence(null)`), or rendering an unknown `kind`.

## CLI — `tabnas-railroad`

```
tabnas-railroad --grammar <module>[#export] [-o <dir>] [formats]
tabnas-railroad -f <model.json> [format]
echo '<model.json>' | tabnas-railroad - [format]
```

### Modes

| Flag | Alias | Meaning |
|---|---|---|
| `--grammar <m>[#export]` | `-g` | `require` module `<m>`, find its grammar plugin export, install it on a fresh `Tabnas`, introspect it. |
| `--file <path>` | `-f` | Read a saved `GrammarModel` JSON file. |
| `-` | | Read a `GrammarModel` JSON from stdin. |

In grammar mode the plugin export is chosen as: the named `#export`, else
the export matching the bare module name (e.g. `json` for `@tabnas/json`),
else a default/function export, else the first function export.

### Output

| Flag | Alias | Meaning |
|---|---|---|
| `--out <dir>` | `-o` | Write artifacts into `<dir>` (default `./out` in grammar mode). |

When writing (grammar mode, or any mode with `-o`) it writes the selected
formats as `grammar.railroad.json` / `grammar.svg` / `grammar.txt` and logs
`wrote <files> to <dir>/`. Without `-o`, render mode prints **one** format
to stdout.

### Formats

| Flag | Meaning |
|---|---|
| `--json` | Declarative JSON model (`grammar.railroad.json`). |
| `--svg` | Vertical-flow SVG (`grammar.svg`). |
| `--ascii` | Vertical ASCII diagram (`grammar.txt`). |
| `--ascii-plain` | ASCII with plain `\| - +` glyphs (implies `--ascii`). |
| `--text` | Compact per-rule EBNF text (`grammar.txt`). |
| `--help`, `-h` | Print usage. |

Defaults: when **writing**, no format flag means all three (`--json`,
`--svg`, `--ascii`); when **printing** to stdout, the default is `--svg`.
`--ascii` and `--text` both target `grammar.txt`; `--text` wins if both are
given.

### Examples

```bash
tabnas-railroad --grammar @tabnas/json -o diagrams
tabnas-railroad -f diagrams/grammar.railroad.json --ascii
tabnas-railroad --grammar @tabnas/json --text -o /tmp/rr
```

### Programmatic entry

The CLI is also importable; `run` takes a `console` sink so it can be
driven in-process (used by the tests):

```ts
import { run } from '@tabnas/railroad/dist/bin/tabnas-railroad-cli'
await run(process.argv, console)
```

## Scope

This renderer introspects **`@tabnas/parser`** (`Tabnas`) instances only.
The `@tabnas/ini` and `@tabnas/yaml` grammars target the older
`@tabnas/jsonic` engine and are **not yet supported**.
