# Agents Guide — railroad

## What this project is

`@tabnas/railroad` is a **railroad (syntax) diagram generator** for the
[`tabnas`](https://github.com/tabnas/parser) parser. It does **not** parse
anything itself: it **introspects a live `Tabnas` instance that already has
a grammar installed** and emits three artifacts from that grammar —

- a declarative, JSON-serializable **`GrammarModel`** (the interchange
  format: one node tree per rule),
- a vertical-flow **SVG** (one anchored, linked track per rule), and
- a vertical **ASCII** diagram (Unicode box-drawing, or plain `| - +`).

It also ships the **`tabnas-railroad` CLI**. Diagrams bias toward
**verticality** (tall and narrow) so they read on laptops and phones:
sequences run top-to-bottom, choices fan out sideways, optional /
repetition rails run on the side.

This package is itself a **dev tool the other tabnas repos depend on**: it
is the `@tabnas/railroad` dev-only `file:` devDependency they use to
(re)generate the `ts/doc/grammar.{svg,txt}` README diagrams. It is the same
role `@tabnas/debug` plays for introspection — a tool, not a grammar.

There **is** a Go port in `go/` (`package tabnasrailroad`, plus
`cmd/tabnas-railroad`). `ts/` stays **canonical**; `go/` tracks it.

## Repository map

| Path | What it is |
|---|---|
| [`ts/`](ts/) | The canonical package — `@tabnas/railroad`. |
| [`go/`](go/) | The Go port — `package tabnasrailroad` (`model.go`, `extract.go`, `svg.go`, `ascii.go`, `railroad.go`) + the `cmd/tabnas-railroad` CLI, mirroring the TS files one-for-one. `const Version` in `go/model.go` tracks the npm version. |
| [`test/spec/`](test/spec/) | Shared cross-runtime `*.tsv` fixtures (`node-text.tsv`, `node-ascii.tsv`), run by BOTH runtimes. See [`test/AGENTS.md`](test/AGENTS.md). |
| [`ts/src/model.ts`](ts/src/model.ts) | The `RailroadNode` tagged union + `GrammarModel` envelope, node constructors (`Terminal`/`NonTerminal`/`Comment`/`Skip`/`Sequence`/`Choice`/`Optional`/`OneOrMore`/`ZeroOrMore`/`Diagram`), `toText`, `norm`, `nodeEqual`, `RailroadError`. Pure data — the interchange format. |
| [`ts/src/extract.ts`](ts/src/extract.ts) | `extractGrammar(tn)` — reverse-maps a live instance's alt-based rule machine into the model. **The heart of the package.** |
| [`ts/src/svg.ts`](ts/src/svg.ts) | `modelToSvg` / `renderNodeSvg` — vertical-flow SVG renderer. |
| [`ts/src/ascii.ts`](ts/src/ascii.ts) | `modelToAscii` / `renderNodeAscii` — vertical-flow ASCII renderer. |
| [`ts/src/railroad.ts`](ts/src/railroad.ts) | The **plugin entry point** (the package `main`) + bare re-exports. Loading it decorates the instance with `tn.railroad`. |
| [`ts/src/renderer.ts`](ts/src/renderer.ts) | Back-compat re-export surface forwarding the historical `renderer` import paths to `model`/`extract`/`svg`/`ascii`. |
| [`ts/src/bin/tabnas-railroad-cli.ts`](ts/src/bin/tabnas-railroad-cli.ts) | The CLI implementation (`run(argv, console)`). |
| [`ts/bin/tabnas-railroad`](ts/bin/tabnas-railroad) | CLI launcher (the `tabnas-railroad` bin); `require`s `dist/bin/tabnas-railroad-cli` and calls `run`. |
| [`examples/json-grammar.{svg,txt}`](examples/) | Sample output: the `@tabnas/json` grammar rendered, used by the READMEs. |
| `ts/test/*.test.js` | Committed JS tests (not compiled): `railroad.test.js` (node-level + whole-model rule ordering), `grammar.test.js` (extraction + CLI against `@tabnas/json`), `doc-examples.test.js` (runs `// =>` README examples), `parity.test.js` (runs the shared `test/spec/*.tsv`). |
| `go/*_test.go` | `railroad_test.go` / `grammar_test.go` / `parity_test.go` — the ports of the above, plus `TestParityWithTypeScriptModel` against the `go/testdata/ts-json-model.json` snapshot of the TS model. |

## The tabnas engine dependency

This renderer introspects **`@tabnas/parser`** (`Tabnas`) instances, so the
engine is its one runtime tabnas dependency, declared via the standard
**sibling checkout** dev model:

- `@tabnas/parser` is the `peerDependency` (npm >=7 / Node >=24
  auto-installs it; `engines.node` is `">=24"`) and is **also** a `file:`
  devDependency for local builds — both currently pinned to
  `file:../../parser/ts` in `ts/package.json`.
- `@tabnas/json` is a **dev-only** `file:` devDependency used as the **test
  grammar**: the suite installs it on a `Tabnas` instance and asserts the
  extracted model/SVG/ASCII. It is the package's known-good fixture
  grammar, not a runtime dep.
- `@tabnas/debug` is a declared `file:` devDependency (the usual sibling),
  but nothing in `src/` or `test/` references it yet — there is no
  `debug.model()` composition test here.

Note the **inversion** versus a grammar plugin: a grammar repo lists
`railroad` as a dev tool; here `railroad` lists `json` as the grammar it
renders in tests. Clone `parser` and `json` (and the rest of the closure
for CI) as siblings of this repo and build their TS first; CI does this for
you (see below).

## How extraction works (the non-obvious core)

`extractGrammar(tn)` reads the instance loosely — `tn.rule()` for the rule
set and `tn.internal().config` for the resolved config — and reverse-maps
the engine's **alt-based rule machine** (each rule has `open` / `close`
alternatives) into railroad constructs. The mapping is documented at the
top of `extract.ts`; the load-bearing rules an agent must keep in mind:

- An **open alt** consumes `sN - b` leading token positions as terminals,
  then a `p:` push becomes a nonterminal; a pure peek (`b == sN`) consumes
  nothing. Several open alts become a **choice**.
- A **close alt** with `r: <self>` (plus a guard token) becomes a
  **repetition** (`OneOrMore`, the guard token on the return path);
  `r: <other>` is a continuation; a token-consuming close with no backup is
  the rule's own closing terminal; a `b:` backup close or pure pop /
  end-of-source is dropped (it belongs to the parent).
- **Synthetic helper rules** (name contains `$` or matches `_gen\d`) are
  **inlined**, not emitted as their own rule; `__start__` is unwrapped to
  the real entry rule.
- A **normalization pass** (`factor`, on by default) factors common
  prefix/suffix across choice branches and turns an empty branch into
  `Optional`.

Because the model is pure data, **the SVG and ASCII are fully reproducible
from the JSON alone** — `grammar.test.js` asserts a JSON round-trip yields
byte-identical SVG/ASCII. Keep it that way: don't let a renderer read
anything off the live instance.

### Token legend and ignored-token key

Token labels that aren't self-explanatory punctuation get a **legend**
(`model.legend`), and the lexer's **IGNORE set** (whitespace, newlines,
comments — tokens that never appear in any rule) is reported separately as
`model.ignored`. Both are rendered into the SVG ("Tokens" / "Ignored
tokens" keys) and ASCII. The meaning of a token is resolved in priority
order: a **grammar-supplied description** (`cfg.tokenDesc`, set by a plugin
via the `config.modify` hook) wins, then the built-in `CANON` table of
standard tabnas/jsonic token names, then an engine-derived meaning
(regex source / reverse-resolved fixed literal / owning token set). If you
add tokens to a grammar and want good legends, attach `tokenDesc` entries
rather than editing `CANON` here.

## The plugin API

Loading `railroad` decorates the instance with `tn.railroad` — a
**callable** plus helpers. Decoration is **lazy**: every helper re-reads
the instance's current grammar when called, so plugin install order does
not matter.

- `tn.railroad()` / `tn.railroad.toJson()` / `tn.railroad.extract()` — the
  `GrammarModel` for this instance.
- `tn.railroad.toSvg(opts?)` — whole-grammar SVG.
- `tn.railroad.toAscii(opts?)` — whole-grammar ASCII (`{ ascii: true }` for
  plain `| - +`).
- `tn.railroad.renderNode` / `renderNodeAscii` / `renderNodeText`, plus the
  model constructors (`Diagram`, `seq`, `choice`, `opt`, `plus`, `star`,
  `t`, `n`, `comment`, `skip`) for instance-free use.

The same functions are exported bare from the package (`extractGrammar`,
`modelToSvg`, `modelToAscii`, `toText`, the constructors, `RailroadError`)
for use without a live instance.

## The CLI

`bin/tabnas-railroad` (the `tabnas-railroad` bin) `require`s
`dist/bin/tabnas-railroad-cli` and calls `run(process.argv, console)`. Two
modes:

- **grammar mode** (`--grammar <module>[#export]`, alias `-g`): `require`
  the module, find its grammar plugin export, install it on a fresh
  `Tabnas`, introspect, and write `grammar.railroad.json` + `grammar.svg` +
  `grammar.txt` into `-o <dir>` (default `./out`).
- **render mode** (`-f <model.json>`, or a **bare `-`** argument for stdin —
  note it is `… | tabnas-railroad - --text`, *not* `-f -`, which would look
  for a file literally named `-`): read a saved `GrammarModel` and render
  one format to stdout (default SVG; `--json`, `--svg`, `--ascii`,
  `--ascii-plain`, `--text`).

`run(argv, console)` takes the `console` sink as an argument so the tests
drive it in-process (`grammar.test.js` passes a capturing fake console);
keep that signature. This is how `examples/json-grammar.{svg,txt}` and the
downstream repos' `ts/doc/grammar.*` are regenerated, e.g.
`tabnas-railroad --grammar @tabnas/json -o examples`.

The Go port ships the same CLI as `go/cmd/tabnas-railroad`, with
`run(argv, stdin, stdout, stderr) int` as the testable seam. All the flags
above behave identically; verified byte-for-byte against the TS CLI for
`--json/--svg/--ascii/--ascii-plain/--text` and bare-`-` stdin.

## Scope / known limitations

This renderer only introspects **`@tabnas/parser`** (`Tabnas`) instances.
The `@tabnas/ini` and `@tabnas/yaml` grammars target the older
`@tabnas/jsonic` engine and are **not yet supported** — a jsonic
introspection adapter is deferred. `grammar.test.js` validates extraction
against `@tabnas/json` only.

## Build & test

From `ts/`:

```bash
npm install            # auto-installs the @tabnas/parser peer; resolves file: siblings
npm run build          # tsc --build src  (emits dist/)
npm test               # node --enable-source-maps --test test/**/*.test.js
```

`npm run build` compiles **`src` only** — the `test/*.test.js` files are
**committed JS, not compiled** — and the tests `require('..')` →
`dist/railroad.js`, so **you must build before testing** (a stale or
missing `dist/` makes the suite fail or run old code). `npm run reset`
(`clean && npm i && build && test`) is the from-clean path.

The repo-root [`Makefile`](Makefile) drives **both** runtimes:
`make build|test|clean` fan out to `build-ts`/`build-go`,
`test-ts`/`test-go`, `clean-ts`/`clean-go`. `make publish-ts` runs the tests
then `npm publish --access public` at the `package.json` version;
`make publish-go V=x.y.z` injects `V` into `const Version` in `go/model.go`,
commits, and tags `go/vX.Y.Z` (`make tags-go` lists those tags).

From `go/`: `go build ./...` and `go test ./...`.

## CI

`.github/workflows/ci.yml` is a thin caller of the org-standard reusable
workflow `tabnas/.github/.github/workflows/polyglot-ci.yml@main`, passing
`deps: "parser debug json abnf"` (the upstream closure cloned as siblings).
It runs on push/PR to `main`, and covers both the TS and Go sides. The
workflow file is promoted by a maintainer via
`tabnas/admin rollout/apply-ci-folders.sh` — session credentials cannot
write `.github/workflows/*` (admin `DECISIONS.md` ADR-8), so edit it there,
not here. `.github/workflows/release.yml` handles releases.

The `@tabnas/json` clone is what makes the grammar-extraction and CLI tests
runnable in CI; `npm test` includes them because `json` is a `file:`
devDependency.

## Rule order

The two runtimes produce **byte-identical** ASCII and SVG for the same
`GrammarModel` — verified by feeding the TS-extracted model through the Go
renderers — and both now put rules in the model in **grammar declaration
order**.

- **TS** iterates `Object.keys(rsm)`; a JS object literal keeps insertion
  order for free (`val, map, list, pair, elem` for `@tabnas/json`).
- **Go** asks the engine. `@tabnas/parser`'s Go port stamps every rule spec
  with a definition index (`RuleSpec.Def`) at registration and exposes the
  walk as `(*Tabnas).RuleNames()` / `Rules()`. `declaredUserRules` in
  `extract.go` filters that to user rules and it becomes
  `GrammarModel.RuleOrder`, which `MarshalJSON`, `UnmarshalJSON` (which
  recovers key order from the raw JSON) and `orderRules` in `svg.go` already
  carried. `extract.go` no longer sorts; the old `sortedUserRules` is gone.

This is a **new engine API**, not in a published release. `go/go.mod` still
requires `github.com/tabnas/parser/go v0.6.1`, which has no `RuleNames`, so
`go/` compiles only against the sibling engine — the repo `go.work` locally,
the `go work` step in the CI workflow. `GOWORK=off go build ./...` fails
until `go/go.mod` is bumped past the release that carries `RuleNames`. That
bump is the one remaining step.

### Caveat: order is only as good as the grammar's declaration

A Go map has no order for the engine to recover, so a plugin registering a
bare `GrammarSpec` **must** state `GrammarSpec.RuleOrder`; without it the
engine stamps the rules in sorted-name order and `RuleNames()` reports
alphabetical. `GrammarText` fills `RuleOrder` in automatically from the
source key order, so text grammars need nothing.

`@tabnas/json`'s Go plugin (`RegisterJSONGrammar`) does not yet set
`RuleOrder`, so the extracted Go model for the JSON grammar is still
`elem, list, map, pair, val` where TS gives `val, map, list, pair, elem`.
That is the **json** repo's to fix (add `RuleOrder` to its `GrammarSpec`),
not railroad's — railroad now reports faithfully whatever the grammar
declared. `examples/json-grammar.{svg,txt}` are TS-generated and already
show declaration order.

Note `TestParityWithTypeScriptModel` compares rules **by name**, so it does
not assert order. The ordering contract is pinned by
`TestExtractionHonoursDeclarationOrder` (extraction) and
`TestRenderersHonourDeclaredRuleOrder` (rendering) in Go, and the
`whole-model rule ordering` block in `ts/test/railroad.test.js`.
