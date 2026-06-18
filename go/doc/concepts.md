# Concepts — how `tabnasrailroad` works and why (Go)

This explains the design of the Go port: the relationship to the parsing
engine, how a live grammar is reverse-mapped into a diagram model, why the
layout is vertical, and — importantly — how the Go port differs from the
canonical TypeScript implementation it tracks.

For tasks, see [guide.md](guide.md) and [tutorial.md](tutorial.md); for the
exact surface see [reference.md](reference.md).

## A renderer, not a parser

`tabnasrailroad` produces no parse trees and consumes no input text. Point
it at a live [`tabnas`](https://github.com/tabnas/parser) parser instance
that already has a grammar installed, and it draws *that grammar*.

It is a Go port of the canonical TypeScript package
[`@tabnas/railroad`](../../ts). The TS side is authoritative; the Go side is
held to match it for the same grammar.

## The pipeline: instance → model → render

Three stages, with a pure-data boundary between extraction and rendering:

```
live *tabnas.Tabnas instance
        │   extract.go: ExtractGrammar(tn)
        ▼
   *GrammarModel  (pure JSON — the interchange format)
        │   svg.go / ascii.go / model.go: ToText
        ▼
   SVG  /  ASCII  /  text
```

The **`GrammarModel`** is the load-bearing seam: plain JSON-serializable
data, a tagged union of nodes (`terminal`, `nonterminal`, `seq`, `choice`,
`optional`, `oneOrMore`, `zeroOrMore`, `comment`, `skip`, `diagram`), one
node tree per rule, plus the entry rule, a token legend, and the
ignored-token set.

Because the renderers read **only** the model and never the live instance,
SVG and ASCII are fully reproducible from the JSON alone. The tests assert
that a model round-tripped through `encoding/json` yields identical SVG and
ASCII. The CLI's render mode (`-f model.json`) relies on exactly this.

## Grammar introspection: reversing the rule machine

This is the non-obvious core, in `extract.go`.

A tabnas grammar is not stored as an EBNF tree. The engine compiles each
rule into an **alt-based rule machine**: every rule has a list of `open`
alternatives and `close` alternatives. `ExtractGrammar` reads the rule set
(`tn.RSM()`) and the resolved config (`tn.Config()`) and **reverse-maps**
those alts back into railroad constructs:

- **Open alt.** Its first `sN - b` token positions are consumed terminals; a
  push (`P`) appends the pushed rule as a nonterminal. A pure peek
  (`b == sN`) renders only the reference. Several open alts become a
  **choice**.
- **Close alt.** `R == <self>` (plus a guard token) is a **repetition** —
  `OneOrMore` with the guard token on the return rail (where json's `,`
  separator comes from). `R == <other>` is a continuation. A token-consuming
  close with no backup and no `R` is the rule's own **closing terminal**
  (json's `}` / `]`). A backup close (`b > 0`), end-of-source, or pure pop is
  **dropped** (it belongs to the parent).
- **Synthetic helper rules** (names containing `$` or matching `_gen\d`) are
  **inlined**; `__start__` is unwrapped to the real entry rule.

For the json grammar this yields five clean rules:

```text
val  = (map | list | "VAL")
map  = "{" [pair] "}"
list = "[" [elem] "]"
pair = "KEY" ":" val+ /* "," */
elem = val+ /* "," */
```

A **normalization pass** (`factor`, on by default; disable with
`NoFactor: true`) then turns an empty choice branch into `Optional` (so
`map` gets its `[pair]`), factors common prefix/suffix across choice
branches, and dedups identical branches.

## Tokens: labels, legend, and the ignored set

Diagram boxes show labels, but the engine works in numeric token ids
(`Tin`s). The extractor resolves each id to the most readable label it can
(a fixed literal `{`/`:`/`}`, a named token-set name `KEY`/`VAL`, or a regex
source) and records a **legend** entry for non-punctuation labels. A token's
meaning is resolved in priority order: a grammar-supplied description
(`TokenDesc`), then the built-in `canon` table of standard token names, then
an engine-derived meaning. The lexer's **IGNORE set** (whitespace, newlines,
comments — `SP`, `LN`, `CM` for json) never appears in a rule and is
reported separately as `Ignored`.

## Why vertical flow

Most railroad tools lay grammars out left-to-right and become very wide.
`tabnasrailroad` runs flow **top to bottom**: sequences stack vertically,
choices fan out sideways, optional/repetition rails run on the side. The
output is tall and narrow, so it fits a phone or a documentation column. The
SVG is asserted taller than wide.

Both renderers share the flow model. Each node measures to a small layout
record (width, height, entry/exit rail column); parents stack and connect
children's rails. The SVG renderer paints onto a coordinate plane; the ASCII
renderer paints onto a character grid, tracking rail-direction bits per cell
so junctions resolve to the right box-drawing glyph automatically (or to
`| - +` in plain mode).

## Whole-grammar assembly

`ModelToSvg` / `ModelToAscii` render the entry rule first, then the rest. In
SVG, each rule is a titled, anchored track (`<g id="rule">`) and every
nonterminal box links to the referenced rule (`<a href="#rule">`); tracks
pack into two balanced columns. In ASCII, each rule is a titled block,
stacked vertically. Both append the "Tokens" / "Ignored tokens" keys.

## Differences from the TS version

The TypeScript implementation in [`../../ts`](../../ts) is canonical; this
port matches it where it counts and diverges where Go idiom or the Go engine
shape requires. The cross-language contract (checked by `parity_test.go`
against `testdata/ts-json-model.json`) is: **same `start`, same per-rule node
trees, same legend, same ignored set, same `meta.engine`**. Rule-map key
order and raw JSON key order are explicitly **not** part of the contract.

Concrete differences:

- **No bare-string terminals.** TS `Item = RailroadNode | string` lets
  `Sequence('GET', ...)` coerce a string to a terminal. Go terminals are
  always explicit `*RailroadNode`s built with `Terminal(...)`.
- **`Choice` returns an error; `MustChoice` panics.** TS's single `Choice`
  throws on zero branches. Go splits this into `Choice(...) (*RailroadNode,
  error)` and the ergonomic `MustChoice(...) *RailroadNode`.
- **`SkipNode`, not `Skip`.** Renamed to avoid clashing with the `KindSkip`
  constant.
- **Explicit `rep` argument.** TS `OneOrMore(item, rep?)` makes `rep`
  optional; Go `OneOrMore(item, rep)` always takes it — pass `nil` for none.
- **Decoration, not a callable.** TS decorates the instance with a callable
  `tn.railroad`. Go has no callable values with attached methods, so the
  plugin stores a `*RailroadApi` under `DecorationName` (`"railroad"`),
  retrieved with `Of(tn)`. `Of` also binds a fresh API when the plugin was
  not loaded.
- **`ToAscii` takes a required `AsciiOptions`.** TS's `toAscii(opts?)` is
  fully optional; the Go method signature is
  `ToAscii(asciiOpts AsciiOptions, opts ...*ExtractOptions)` — pass
  `AsciiOptions{}` for the default.
- **Options struct shapes.** `ExtractOptions.NoFactor` is the inverse of TS's
  `factor` (default behaviour is identical: factoring on). `AsciiOptions.Plain`
  is TS's `ascii: true`. `SvgOptions.LinkFor` returns `(string, bool)` rather
  than `string | undefined`.
- **Deterministic rule order via `RuleOrder`.** Go maps are unordered, so the
  model carries a `RuleOrder` slice and the custom `MarshalJSON` emits rules
  in that order. TS relies on JS object insertion order instead.
- **Token-set name recovery.** The TS extractor reads the raw `#KEY` / `#VAL`
  token-set name strings straight off each alt. The Go engine does not retain
  those strings on the live `RuleSpec` — it resolves them to `[]Tin` sets — so
  the Go extractor recovers a readable set name by matching the resolved tin
  set against the instance's named token sets, disambiguating identical sets
  (e.g. `KEY` vs `VAL`) by **position role**: a slot immediately followed by a
  colon is a map key. The candidate names and their order come from
  `ExtractOptions.TokenSetNames` (default `["VAL", "KEY"]`); per-token human
  descriptions come from `ExtractOptions.TokenDesc` (the analog of the TS
  `cfg.tokenDesc` hook). The end result matches the TS legend.
- **CLI grammar resolution is static.** TS grammar mode `require`s any module
  and finds its plugin export. Go has no dynamic loading, so `--grammar`
  resolves a fixed built-in set (currently `json`). Render mode is fully
  general in both.

## Design trade-offs

- **Pure-data model over a clever renderer** — buys reproducibility, a stable
  interchange format, a trivial CLI render mode, and the cross-language
  parity above.
- **Heuristic reverse-mapping, not a formal inverse** — the alt machine is
  lower-level than EBNF, so the mapping is documented heuristics tuned and
  tested against `@tabnas/json`. It aims for a *readable* diagram.
- **Verticality over density** — tall-and-narrow trades compactness for
  small-screen readability.
- **Engine scope** — only `tabnas` parser instances are introspected.
