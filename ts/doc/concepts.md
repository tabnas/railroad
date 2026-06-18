# Concepts — how `@tabnas/railroad` works and why

This explains the design: the relationship to the parsing engine, how a
live grammar is reverse-mapped into a diagram model, and why the layout is
biased toward verticality. It is background, not a task list — for those see
[guide.md](guide.md) and [tutorial.md](tutorial.md).

## A renderer, not a parser

`@tabnas/railroad` produces no parse trees and consumes no input text. It is
a **dev tool that the other tabnas repos depend on**: point it at a live
[`tabnas`](https://github.com/rjrodger/tabnas) parser instance that already
has a grammar installed, and it draws *that grammar*.

This is the inverse of the usual dependency direction. A grammar package
(like `@tabnas/json`) is a runtime thing; `railroad` is a `file:` dev
dependency those packages use to regenerate the diagrams in their READMEs.
It plays the same role for grammars that `@tabnas/debug` plays for
introspection — a tool, not a grammar.

## The pipeline: instance → model → render

There are three stages, each in its own module, with a pure-data boundary
between them:

```
live Tabnas instance
        │   extract.ts: extractGrammar(tn)
        ▼
   GrammarModel  (pure JSON — the interchange format)
        │   svg.ts / ascii.ts / model.ts:toText
        ▼
   SVG  /  ASCII  /  text
```

The **`GrammarModel`** is the load-bearing seam. It is plain
JSON-serializable data — a tagged union of nodes (`terminal`,
`nonterminal`, `seq`, `choice`, `optional`, `oneOrMore`, `zeroOrMore`,
`comment`, `skip`, `diagram`), one node tree per rule, plus the entry rule,
a token legend, and the ignored-token set.

Because the renderers read **only** the model and never the live instance,
the SVG and ASCII are fully reproducible from the JSON alone. The test suite
asserts that a model round-tripped through `JSON.stringify` yields
byte-identical SVG and ASCII. This is a deliberate invariant: you can
extract once, store the JSON, and render any time later — the CLI's render
mode (`-f model.json`) is exactly this.

## Grammar introspection: reversing the rule machine

This is the non-obvious core, in `extract.ts`.

A tabnas grammar is not stored as an EBNF tree. The engine compiles each
rule into an **alt-based rule machine**: every rule has a list of `open`
alternatives (what it can start with) and `close` alternatives (how it
ends, loops, or hands control back). `extractGrammar` reads the rule set
(`tn.rule()`) and the resolved config (`tn.internal().config`) and
**reverse-maps** those alts back into railroad constructs. The mapping:

- **Open alt.** Its first `sN - b` token positions are consumed terminals;
  a `p:` push appends the pushed rule as a nonterminal. A pure peek
  (`b == sN`, consuming nothing) renders only the reference. Several open
  alts become a **choice**.
- **Close alt.** `r: <self>` (plus a guard token) is a **repetition** —
  `OneOrMore` with the guard token on the return rail (that is where json's
  `,` separator comes from). `r: <other>` is a **continuation** appended in
  sequence. A close alt that consumes a token with no backup and no `r` is
  the rule's own **closing terminal** (json's `}` / `]`). A backup close
  (`b > 0`) leaves the token for the parent and is **dropped**; an
  end-of-source or pure pop is also dropped.
- **Synthetic helper rules** — names containing `$` or matching `_gen\d` —
  are **inlined** at their reference site rather than emitted as their own
  rule. The `__start__` wrapper is unwrapped to the real entry rule.

The result, for `@tabnas/json`, is five clean rules:

```text
val  = (map | list | "VAL")
map  = "{" [pair] "}"
list = "[" [elem] "]"
pair = "KEY" ":" val+ /* "," */
elem = val+ /* "," */
```

— which is recognizable as JSON even though none of that structure was
stored that way. The extractor is deliberately **loose** about the
introspection object: it reads `tn` through the documented `rule()` /
`internal().config` shape and never hard-depends on deeper internals, so an
engine refactor that preserves that shape does not break it.

### Normalization (the `factor` pass)

After reverse-mapping, a normalization pass (`factor`, on by default)
cleans up the tree:

- **Empty branch → optional.** A choice with one empty (skip) branch
  becomes `Optional` of the rest. This is how `map = "{" [pair] "}"` gets
  its `[pair]`: the rule can have a pair or nothing.
- **Prefix/suffix factoring.** Common leading and trailing elements shared
  by every branch of a choice are hoisted out, so `(a x | a y)` becomes
  `a (x | y)`. This keeps diagrams narrow.
- **Dedup.** Identical branches collapse.

Pass `{ factor: false }` to skip it and see the raw reverse-mapped tree,
which sits closer to the engine's alt structure.

## Tokens: labels, legend, and the ignored set

Diagram boxes show **labels**, but the engine works in numeric token ids
(`tin`s). The extractor resolves each id to the most readable label it can:
a reverse-resolved fixed literal (`{`, `:`, `}`), a named token-set name
(`KEY`, `VAL`), or a regex source — and records a **legend** entry for
labels that are not self-explanatory punctuation.

A token's *meaning* is resolved in priority order:

1. a **grammar-supplied description** — a plugin can attach `cfg.tokenDesc`
   via the engine's `config.modify` hook, and railroad reads it straight
   off the live config;
2. the built-in **`CANON`** table of standard tabnas/jsonic token names
   (`OB`, `CL`, `NR`, `ST`, ...);
3. an **engine-derived** meaning — a regex source, a reverse-resolved fixed
   literal, or the token set the id belongs to.

If you add tokens to a grammar and want good legends, attach `tokenDesc`
entries rather than editing this package.

Separately, the lexer's **IGNORE set** (whitespace, newlines, comments —
tokens silently skipped between meaningful tokens) never appears in any
rule, so it cannot be drawn. The extractor reports it on its own as
`model.ignored`, rendered as an "Ignored tokens" key. For `@tabnas/json`
that is `SP`, `LN`, `CM`.

The legend is pruned to only the labels that actually appear in the final
diagram, so it never lists tokens you cannot see.

## Why vertical flow

Most railroad-diagram tools lay grammars out **horizontally** — flow runs
left to right, and a rule with several alternatives becomes very wide. That
is fine on a wide desktop monitor but scrolls badly on a laptop and is
nearly unusable on a phone.

`railroad` biases the other way: flow runs **top to bottom**. Sequences
stack vertically, choice branches fan out sideways (but a choice is usually
shallow), and optional / repetition rails run parallel on the side. The
output is **tall and narrow** — it fits a phone or a documentation column
and scrolls naturally. The SVG document is asserted to be taller than wide.

Both renderers share this flow model. Each node measures to a small layout
record (width, height, entry/exit rail position) and draws itself at a given
origin; parents stack and connect children's rails. The SVG renderer paints
onto a coordinate plane; the ASCII renderer paints onto a character grid,
tracking rail direction bits per cell so junctions resolve to the right
box-drawing glyph (`├`, `┬`, `┼`, ...) automatically — or to `| - +` in
plain mode.

## Whole-grammar assembly

`modelToSvg` / `modelToAscii` render the entry rule first, then the rest.
In SVG, each rule becomes a titled, anchored track (`<g id="rule">`) and
every nonterminal box links to the referenced rule's track
(`<a href="#rule">`), so the diagram is navigable; tracks are packed into
two balanced newspaper-style columns to use horizontal space. In ASCII,
each rule is a titled block, stacked vertically. Both append the "Tokens"
and "Ignored tokens" keys at the end.

## Design trade-offs

- **Pure-data model over a clever renderer.** Keeping the model free of any
  reference to the live instance costs a little (the extractor must resolve
  every label up front) but buys reproducibility, a stable interchange
  format, a trivial CLI render mode, and easy cross-language parity.
- **Heuristic reverse-mapping, not a formal inverse.** The alt machine is
  lower-level than EBNF, so the mapping is a set of documented heuristics
  tuned and tested against real grammars (`@tabnas/json`). It aims for a
  *readable* diagram, not a provably exact inverse of the compiler.
- **Verticality over density.** Tall-and-narrow trades some compactness for
  readability on small screens — the stated goal.
- **Engine scope.** Only `@tabnas/parser` instances are introspected; the
  older `@tabnas/jsonic` grammars (`ini`, `yaml`) are out of scope until a
  jsonic adapter exists.
