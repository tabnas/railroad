# Reference — `tabnasrailroad` (Go)

The complete public surface of the Go port: the plugin and API, the
package-level functions, the option and model types, and the
`cmd/tabnas-railroad` CLI. Dry and complete.

```go
import tabnasrailroad "github.com/tabnas/railroad/go"
```

Package name: `tabnasrailroad`. Module path:
`github.com/tabnas/railroad/go`. Current `Version` constant: `"0.1.0"`.

## The plugin and API

### `Plugin(tn *tabnas.Tabnas, opts map[string]any) error`

The tabnas plugin entry point. Decorates `tn` under the key
`DecorationName` (`"railroad"`) with a `*RailroadApi`. Decoration is lazy —
each helper re-reads the instance's current grammar on call, so install
order does not matter. Derived instances (`tn.Derive()`) inherit it.

```go
const DecorationName = "railroad"
```

### `Of(tn *tabnas.Tabnas) *RailroadApi`

Returns the `*RailroadApi` installed by `Plugin`. If the plugin was not
loaded, it returns a freshly bound API for `tn`, so you can use the helpers
without calling `Plugin` first. Never returns nil.

### `*RailroadApi`

| Method | Signature | Returns |
|---|---|---|
| `Extract` | `(opts ...*ExtractOptions) *GrammarModel` | the grammar model |
| `ToJson` | `(opts ...*ExtractOptions) *GrammarModel` | alias of `Extract` |
| `ToSvg` | `(opts ...*ExtractOptions) (string, error)` | whole-grammar SVG |
| `ToAscii` | `(asciiOpts AsciiOptions, opts ...*ExtractOptions) (string, error)` | whole-grammar ASCII |
| `RenderNode` | `(node *RailroadNode, opts ...SvgOptions) (string, error)` | single-node SVG |
| `RenderNodeAscii` | `(node *RailroadNode, opts ...AsciiOptions) (string, error)` | single-node ASCII |
| `RenderNodeText` | `(node *RailroadNode) (string, error)` | single-node EBNF text |

Note `ToAscii` takes a **required** leading `AsciiOptions` (use
`AsciiOptions{}` for Unicode), then variadic extract options.

## Package-level functions (instance-free)

### `ExtractGrammar(tn *tabnas.Tabnas, opts ...*ExtractOptions) *GrammarModel`

Introspects a live `*tabnas.Tabnas` instance into a `*GrammarModel`. This is
what `api.ToJson()` calls; `ExtractGrammar(tn)` and `Of(tn).ToJson()`
marshal to identical JSON for the same instance. **The heart of the
package.**

### `ModelToSvg(model *GrammarModel, opts ...SvgOptions) (string, error)`

Render a whole model to a standalone SVG document: a vertical stack of
titled, anchored rule tracks (`<g id="rule">`), nonterminal boxes linking to
the referenced rule (`<a href="#rule">`), plus "Tokens" / "Ignored tokens"
key blocks when present. Height-biased (taller than wide). The `linkFor`
hook is supplied internally.

### `ModelToAscii(model *GrammarModel, opts ...AsciiOptions) (string, error)`

Render a whole model to a vertical ASCII diagram, one titled block per rule,
followed by the key blocks when present. `AsciiOptions{Plain: true}` selects
plain `| - +` glyphs.

### `RenderNodeSvg(node *RailroadNode, opts ...SvgOptions) (string, error)`

Render a single node to its own standalone SVG with entry/exit rail caps.

### `RenderNodeAscii(node *RailroadNode, opts ...AsciiOptions) (string, error)`

Render a single node to an ASCII block.

### `ToText(node *RailroadNode) (string, error)`

Compact EBNF-ish rendering of a single node:

| Node | Text |
|---|---|
| `Terminal("x")` | `"x"` (JSON-quoted) |
| `NonTerminal("x")` | `x` |
| `Comment("x")` | `/* x */` |
| `SkipNode()` | `` (empty) |
| `Sequence(a, b)` | `a b` (empty parts dropped) |
| `MustChoice(a, b)` | `(a \| b)` |
| `Optional(a)` | `[a]` |
| `OneOrMore(a, nil)` | `a+` |
| `OneOrMore(a, sep)` | `a+ /* sep */` |
| `ZeroOrMore(a, nil)` | `{a}` |
| `ZeroOrMore(a, sep)` | `{a} /* sep */` |

```go
t, _ := tabnasrailroad.ToText(tabnasrailroad.Sequence(
	tabnasrailroad.Terminal("a"), tabnasrailroad.Terminal("b")))
// t == `"a" "b"`
```

## Node constructors

Each returns a `*RailroadNode`. Go terminals are always explicit — there is
no bare-string coercion (the TS `Item = RailroadNode | string` collapses to
`*RailroadNode` here).

| Constructor | Signature |
|---|---|
| `Terminal(text string) *RailroadNode` | terminal (literal / named token) |
| `NonTerminal(text string) *RailroadNode` | nonterminal (rule reference) |
| `Comment(text string) *RailroadNode` | inline comment |
| `SkipNode() *RailroadNode` | bypass (empty) node |
| `Sequence(items ...*RailroadNode) *RailroadNode` | sequence |
| `Choice(items ...*RailroadNode) (*RailroadNode, error)` | choice; errors on zero branches |
| `MustChoice(items ...*RailroadNode) *RailroadNode` | choice; panics on zero branches |
| `Optional(item *RailroadNode) *RailroadNode` | optional |
| `OneOrMore(item, rep *RailroadNode) *RailroadNode` | one-or-more (rep on the return rail; pass `nil` for none) |
| `ZeroOrMore(item, rep *RailroadNode) *RailroadNode` | zero-or-more |
| `Diagram(items ...*RailroadNode) *RailroadNode` | top-level wrapper |

`SkipNode` is named differently from TS's `Skip` to avoid clashing with the
`KindSkip` constant. `Choice`/`MustChoice` replace TS's single throwing
`Choice`. `OneOrMore`/`ZeroOrMore` take an explicit `rep` argument (TS makes
it optional) — pass `nil` for no return-rail node.

## Types

### `RailroadNode`

A struct port of the TS discriminated union. Which fields are populated
depends on `Kind`:

```go
type RailroadNode struct {
	Kind  NodeKind        // the tag
	Text  string          // terminal / nonterminal / comment
	Items []*RailroadNode // seq / choice / diagram
	Item  *RailroadNode   // optional / oneOrMore / zeroOrMore
	Rep   *RailroadNode   // oneOrMore / zeroOrMore (optional)
}
```

`NodeKind` is a string alias with constants `KindTerminal`,
`KindNonTerminal`, `KindComment`, `KindSkip`, `KindSeq`, `KindChoice`,
`KindOptional`, `KindOneOrMore`, `KindZeroOrMore`, `KindDiagram`.

Custom `MarshalJSON`/`UnmarshalJSON` reproduce the TS object shapes exactly:
each kind serializes only its relevant fields (so a terminal is
`{"kind":"terminal","text":"x"}`, a skip is `{"kind":"skip"}`), and `Rep` is
omitted when nil.

### `GrammarModel`

```go
type GrammarModel struct {
	Start     string                   // entry rule name
	Rules     map[string]*RailroadNode // one node per rule
	RuleOrder []string                 // rule-name order (not JSON-serialized as a field)
	Legend    []LegendEntry            // named-token key (omitempty)
	Ignored   []LegendEntry            // lexer IGNORE set (omitempty)
	Meta      map[string]any           // { "engine": "tabnas" }
}
```

`RuleOrder` makes output deterministic (Go maps are unordered). `MarshalJSON`
emits rules in `RuleOrder`; `UnmarshalJSON` recovers it from the raw JSON key
order. Rule-map key order is **not** part of the cross-language contract —
the Go model orders user rules deterministically, which may differ from the
TS object-key order, but `Start` is always rendered first.

### `LegendEntry`

```go
type LegendEntry struct {
	Token   string `json:"token"`
	Meaning string `json:"meaning"`
}
```

Used for both `Legend` (named tokens that appear in the diagram) and
`Ignored` (the lexer's IGNORE set).

### `ExtractOptions`

```go
type ExtractOptions struct {
	NoFactor      bool              // disable factoring (inverse of TS `factor`, default true)
	Start         string            // override the entry rule
	TokenSetNames []string          // candidate token-set names, preference order (default ["VAL","KEY"])
	TokenDesc     map[string]string // human descriptions for named tokens/sets
}
```

`NoFactor` is the inverse of the TS `factor` flag. `TokenSetNames` and
`TokenDesc` are Go-specific extraction inputs (see [concepts.md](concepts.md)
— the Go engine resolves token-set names differently from TS).

### `SvgOptions`

```go
type SvgOptions struct {
	LinkFor func(name string) (string, bool)   // nonterminal -> (href, ok)
}
```

`ModelToSvg` supplies `LinkFor` itself; you rarely set it.

### `AsciiOptions`

```go
type AsciiOptions struct {
	Plain bool   // true => plain | - + glyphs instead of Unicode
}
```

### `RailroadError`

```go
type RailroadError struct {
	Message string
	Node    any
}
func (e *RailroadError) Error() string
```

Returned (or panicked, via `MustChoice`) for a malformed model: a `Choice`
with no branches, an invalid (nil) node, or an unknown `Kind`. Use
`errors.As(err, &re)` with `var re *tabnasrailroad.RailroadError`.

## CLI — `cmd/tabnas-railroad`

```
tabnas-railroad --grammar <module> [-o <dir>] [formats]
tabnas-railroad -f <model.json> [format]
echo '<model.json>' | tabnas-railroad - [format]
```

### Modes

| Flag | Alias | Meaning |
|---|---|---|
| `--grammar <m>` | `-g` | Build a fresh `Tabnas` with built-in grammar `<m>`, introspect it. |
| `--file <path>` | `-f` | Render a saved `GrammarModel` JSON file. |
| `-` | | Read a `GrammarModel` JSON from stdin. |

Go has no dynamic module loading, so grammar mode resolves a fixed set of
built-in names. Currently supported: `json` (also `@tabnas/json`,
`tabnas/json`, `github.com/tabnas/json/go`). Any `#export` suffix is ignored.
Render mode is fully general.

### Output

| Flag | Alias | Meaning |
|---|---|---|
| `--out <dir>` | `-o` | Write artifacts into `<dir>` (default `./out` in grammar mode). |

When writing (grammar mode, or any mode with `-o`) it writes the selected
formats as `grammar.railroad.json` / `grammar.svg` / `grammar.txt` and logs
`wrote <files> to <dir>/`. Without `-o`, render mode prints **one** format to
stdout.

### Formats

| Flag | Meaning |
|---|---|
| `--json` | Declarative JSON model (`grammar.railroad.json`). |
| `--svg` | Vertical-flow SVG (`grammar.svg`). |
| `--ascii` | Vertical ASCII diagram (`grammar.txt`). |
| `--ascii-plain` | ASCII with plain `\| - +` glyphs (implies `--ascii`). |
| `--text` | Compact per-rule EBNF text (`grammar.txt`). |
| `--help`, `-h` | Print usage. |

Defaults: when **writing**, no format flag means all three; when **printing**
to stdout, the default is `--svg`. `--ascii` and `--text` both target
`grammar.txt`; `--text` wins.

### Examples

```bash
go run ./cmd/tabnas-railroad --grammar json -o diagrams
go run ./cmd/tabnas-railroad -f diagrams/grammar.railroad.json --ascii
go run ./cmd/tabnas-railroad --grammar json --text -o /tmp/rr
```
