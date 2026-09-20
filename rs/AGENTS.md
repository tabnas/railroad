# Agents Guide — rs/

The Rust port of the canonical TypeScript in [`../ts`](../ts). Read
[`../AGENTS.md`](../AGENTS.md) first: it holds the cross-runtime rules
(what extraction does, the rule-order contract, the token legend), and
this file only covers what is specific to this crate.

## Layout

| Path | |
|---|---|
| `src/lib.rs` | `VERSION`, the plugin (`plugin`, `railroad`, `installed`), the bound API (`of`, `RailroadApi`), the re-exports |
| `src/model.rs` | `RailroadNode`, `GrammarModel`, `LegendEntry`, `RailroadError`, the constructors, `to_text`, `node_equal` |
| `src/extract.rs` | `extract_grammar` and `ExtractOptions`: the live instance to a model |
| `src/svg.rs` | `model_to_svg`, `render_node_svg`, `SvgOptions` |
| `src/ascii.rs` | `model_to_ascii`, `render_node_ascii`, `AsciiOptions` |
| `src/cli.rs` | the command, behind the `cli` feature; `run(argv, stdin, stdout, stderr) -> i32` is the testable seam |
| `src/bin/tabnas-railroad.rs` | the launcher |
| `tests/parity_test.rs` | the shared `../test/spec/*.tsv` fixtures, one `tabnas_support::Runner` per file |
| `tests/parity_model_test.rs` | the json grammar's model against `../go/testdata/ts-json-model.json` |
| `tests/grammar_test.rs` | the port of `go/grammar_test.go`, plus the `../examples` byte comparisons |
| `tests/railroad_test.rs` | the port of `go/railroad_test.go` |
| `tests/cli_test.rs` | the port of `go/cmd/tabnas-railroad/main_test.go` |
| `tests/version_test.rs` | the version sites must agree |
| `tests/common/mod.rs` | `build()` (json + railroad), `api()`, `svg_attr()` |

```bash
cargo build --all-targets
cargo test --all-targets
cargo test --doc
cargo clippy --all-targets --all-features -- -D warnings
cargo fmt
```

Three path dependencies on sibling checkouts, none published:
`tabnas` (`../../parser/rs`), `tabnas-json` (`../../json/rs`, the CLI's
built-in grammar and the test grammar) and, dev-only, `tabnas-support`
(`../../support/rs`). `ci/rust/run.sh` refuses to run without all three.

## What is pinned, and by what

The cross-runtime claim is "same model, same bytes", and it is held in
three places, each stronger than the last:

1. `tests/parity_test.rs` runs `node-text.tsv` and `node-ascii.tsv`
   through `tabnas_support::Runner`, so the node-level renderers cannot
   drift from TypeScript or Go without a fixture going red somewhere.
2. `tests/parity_model_test.rs` compares the extracted json model with
   the TypeScript snapshot by value (start, every rule tree, legend,
   ignored, `meta.engine`), then by rule ORDER (which Go cannot assert:
   its json plugin declares none), then by BYTES of the pretty JSON. The
   byte comparison works because `GrammarModel` serializes in the
   TypeScript key order (`start, rules, meta, legend, ignored`) and
   serde_json's pretty printer lays out like `JSON.stringify(m, null, 2)`.
   Keep the field order in `model.rs` exactly that.
3. `tests/grammar_test.rs` holds `model_to_ascii` and `model_to_svg` of
   that model to `../examples/json-grammar.{txt,svg}`, which TypeScript
   generated. A renderer change that is not byte-for-byte the TypeScript
   one fails here, which is the point: the examples are the READMEs'.

## Where the port is not a transliteration

- **`RailroadNode` is an enum.** TypeScript's `norm` rejects a bad node
  at render time; a typed node cannot be bad, so the rejection moved to
  `RailroadNode::from_json` and the renderers are infallible. The
  fixtures' `{"kind":"bogus"}` rows pass because the parity runner's
  parse hook decodes first and a bare `ERROR` accepts any failure.
  `RailroadError` remains for `choice([])` and for decoding.
- **Rule order needs no side channel.** Go carries `RuleOrder` beside
  its map; here `rules` is an `IndexMap` and the engine's
  `rule_specs()` walk is declaration order, so `GrammarModel::rule_order`
  (start hoisted, the rest as declared) is the only ordering code.
- **Set names are recovered from tins.** The engine's `AltSpec.s` is
  `Vec<Vec<Tin>>`; the raw `#KEY` string is gone. `match_token_set`
  matches a position's tins against every named set except `IGNORE`,
  `VAL` then `KEY` first and the rest by name, and prefers `KEY` when
  the next position holds a colon. A set is preferred over a single
  bare token whatever the set's size, which is what makes json's
  one-member `KEY` render as `KEY`. Go does the same with a fixed
  `["VAL","KEY"]` list; the wider default here is closer to TypeScript,
  which renders any grammar-named set. `ExtractOptions::token_set_names`
  narrows it.
- **A function-valued push renders as `/* dynamic */`.** That is the
  TypeScript behaviour (`refNode`); Go drops it. A function-valued `r`
  is treated as no `r` in both, and so here.
- **A function-valued backtrack counts as one**, as `numericBack` does.
- **`token_desc` is an option.** No `cfg.tokenDesc` exists on this
  engine.
- **The CLI is a table.** `grammar_from_name` maps `json` and its
  spellings to `tabnas_json::make()`; anything else is the Go error
  message. The `cli` feature (default) is what pulls `tabnas-json` into
  the library build; `default-features = false` drops both.

## Things that look arbitrary and are not

- `to_text` and the ASCII terminal boxes quote through
  `serde_json::to_string(&str)`, which escapes exactly what
  `JSON.stringify` escapes. Go's `json.Marshal` additionally HTML-escapes
  `<`, `>` and `&`, so a terminal holding one of those renders
  differently in Go; this port matches TypeScript.
- The ASCII choice midpoint is `div_ceil(2)`: `Math.round` rounds a half
  up, and for non-negative integers that is the ceiling.
- Widths are `chars().count()`, as Go counts runes. JavaScript counts
  UTF-16 units, so a label with an astral character would measure one
  wider there. No fixture has one.
- `svg::num` prints a whole number without a decimal point and anything
  else with Rust's shortest round-trip digits, which is what JavaScript
  prints for the values this layout produces (halves and quarters).
- The legend is a `BTreeMap`, so it is sorted by byte order.
  TypeScript sorts with `localeCompare`; for the ASCII token names both
  give the same order.
- `tin_name` treats the engine's `#UNKNOWN` answer as no name, so an
  unregistered tin falls through to `#<tin>` the way an absent
  `cfg.t` entry does in TypeScript.

## The docs

`README.md` is written to [`../docs/STYLE-GUIDE.md`](../docs/STYLE-GUIDE.md)
(no em dashes in prose, no first person singular, no links to an
`AGENTS.md`), but it is not in `ts/scripts/gated-docs.cjs`, so neither
half of the prose gate runs over it. Adding it there is a `ts/` change.

## The README is doctested

`src/lib.rs` includes `README.md` as crate documentation under
`#[cfg(doctest)]`, so `cargo test --doc` compiles and runs every `rust`
fence in the README exactly as it appears on the page. Keep each fence a
complete `fn main` example (no top-level `?`, no hidden `# ` lines), and
expect `readme_examples (line N)` entries in the doctest output, one per
fence.
