# Agents Guide — shared spec fixtures

`spec/*.tsv` holds the cross-runtime conformance fixtures. All three
runtimes auto-discover and run **every** file in this directory, so a
change here affects TypeScript, Go and Rust together — edit with that in
mind.

## Format

Tab-separated, one case per line, with a header row naming the columns.
Blank lines are skipped, and so are comment lines — a line starting with
`#` that contains no tab. (A data row always has at least one tab, so a
`#`-leading source such as a C preprocessor directive still works.)

| Column | Meaning |
|---|---|
| `node` | A railroad node, in the node's own JSON shape — exactly what both runtimes marshal to and unmarshal from. Escapes `\n` `\r` `\t` `\\` are decoded. |
| `text` *or* `ascii` | See below. |
| `opts` | Optional JSON object of renderer options. |

The **second column's header name selects the renderer** run over the node:

- `text` — `toText(node)` / `ToText(node)`.
- `ascii` — `renderNodeAscii(node, opts)` / `RenderNodeAscii(node, ...)`.

Either way the cell holds the rendered string **as JSON** (so newlines and
quotes are unambiguous), or `ERROR` / `ERROR:<code>` when the render must
fail. A code is compared exactly; a bare `ERROR` accepts any failure,
which is what the two `{"kind":"bogus"}` rows use.

The `opts` column uses the TypeScript spelling, `{"ascii":true}` for plain
7-bit output. Go's equivalent field is `AsciiOptions.Plain`; the Go runner
maps the one to the other. Rust's `AsciiOptions.ascii` is the TypeScript
spelling already.

## Who runs what

- TypeScript: `ts/test/parity.test.js` — a `makeRunner(...)` per fixture.
- Go: `go/parity_test.go` — a `support.Runner{...}` per fixture.
- Rust: `rs/tests/parity_test.rs` — a `tabnas_support::Runner` per
  fixture.

One runner per FILE, not one over the directory, because the renderer is
named by the second column's header. Everything else — finding
`test/spec`, reading the file, decoding escapes, the `ERROR:` contract,
the comparison, the `<file>:<line>` in a failure message — comes from
[`@tabnas/support`](https://github.com/tabnas/support) and its Go and Rust
halves, so the three loaders cannot drift from each other either.

All three discover files by directory listing: adding a `.tsv` here runs
it in every runtime without touching any runner. An empty fixture, and a
spec directory with no fixtures in it, both **fail**.

`go/testdata/ts-json-model.json` stays outside this directory: it is a
generated snapshot of the whole extracted model for the @tabnas/json
grammar, checked by `TestParityWithTypeScriptModel` and by
`rs/tests/parity_model_test.rs`, not a set of cases.

SVG output is deliberately not pinned here — the SVG tests assert
well-formedness and layout invariants, not pixel-identical markup.

## Rules

- Prefer adding a fixture here over a one-off in-language assertion when a
  case is expressible as node → rendering. That is what keeps the two
  runtimes honest against each other.
- TypeScript is canonical. If the two runtimes disagree, the TS behaviour is
  the expected value — unless Go has exposed a genuine TS defect, in which
  case fix TS first and pin the corrected behaviour here.
- A new fixture must pass in ALL THREE runtimes: run `go test ./...` (from
  `go/`), `cargo test --all-targets` (from `rs/`) and `npm test` (from
  `ts/`) before considering it done.
