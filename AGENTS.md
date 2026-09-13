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
| [`go/`](go/) | The Go port — `package tabnasrailroad` (`model.go`, `extract.go`, `svg.go`, `ascii.go`, `railroad.go`) + the `cmd/tabnas-railroad` CLI, mirroring the TS files one-for-one. `const VERSION` in `go/model.go` tracks the npm version, and `go/version_test.go` fails the build if it drifts from `ts/package.json`. |
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
`make publish-go V=x.y.z` injects `V` into `const VERSION` in `go/model.go`,
commits, and tags `go/vX.Y.Z` (`make tags-go` lists those tags).

Both runtimes bake in a `VERSION` constant — `const VERSION` in
`go/model.go`, the exported `VERSION` in `ts/src/railroad.ts` — and both are
guarded: `go/version_test.go` and `ts/test/version.test.js` read
`ts/package.json` and fail (never skip) if the constant has drifted from it.
Bump one by hand and the other must follow, or CI goes red.

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

`RuleNames` was a new engine API when this was written, ahead of any
published release, so `go/` compiled only against the sibling engine. That is
no longer true: `go/go.mod` now requires an engine release that carries
`RuleNames`, and `GOWORK=off go build ./...` succeeds against the published
module. The `go.work` and the CI `go work` step are still what pick up local
sibling changes, but they are no longer required to compile.

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

## Releasing

Publishing is **dispatch-driven and runs in CI**, never locally:
[`.github/workflows/release.yml`](.github/workflows/release.yml) publishes
`@tabnas/railroad` to npm over GitHub OIDC trusted publishing (no token,
provenance attached), and a `go/v*` tag is the Go module release —
proxy.golang.org serves it straight from the tag. A local `npm publish` goes
out over a token and bypasses OIDC entirely — do not use it for a release.

### Dispatch it; do not push the tag

**Run the workflow with `workflow_dispatch` on `main`, with the `go` input
true.** That is the path the workflow's own header calls normal, and it is
the only one an agent can take: **a session's credentials cannot push tag
refs — `git push origin ts/v…` fails with HTTP 403**, while branch pushes
from the same credentials succeed. It is a ref-type boundary, not a broken
token or a network fault. Nothing is lost by never touching a tag, because
the workflow creates both tags itself, in one atomic push, *after* npm
accepts the publish. Pushing a tag by hand is the orchestrator's path
(`admin/publish.sh`), not yours.

The steps, in order:

1. Bump all **three** version sites together — `ts/package.json`, `VERSION`
   in `ts/src/railroad.ts` and `const VERSION` in `go/model.go`. Drift is
   caught by `ts/test/version.test.js` and `go/version_test.go`.
2. Verify against the **published** dependencies rather than your checkout.
   The release runner installs fresh from the registry; a working tree
   usually does not, so reproduce that before believing anything:

   ```bash
   (
     cd ts
     rm -f package-lock.json      # gitignored here; pins the old versions
     rm -rf node_modules
     npm install
     npm test
   )
   ```

   **Removing the lockfile is not enough on its own.** It does not touch
   `node_modules`, and the sibling symlinks that make local development work
   (`ts/node_modules/@tabnas/…` pointing at a checkout) survive it — the
   suite then passes against unreleased code while appearing to verify the
   published one. Reinstalling is the part that matters.

   One thing a clean install does **not** isolate:
   `ts/test/doc-examples.test.*` resolves `@tabnas/*` by filesystem path
   (`const TABNAS = path.join(REPO, '..')`), not through `node_modules`. If
   unbuilt sibling checkouts sit beside this repo, those blocks fail with
   `MODULE_NOT_FOUND` no matter what you installed — build the siblings, or
   verify somewhere they are absent.

   `npm test` already compiles here: `ts/package.json` sets `pretest` to
   `npm run build`, which npm runs automatically. No separate build step is
   needed, and adding one just builds twice.

   On the Go side, `GOWORK=off` is necessary and **not sufficient** — it
   disables the workspace and nothing else. A `replace` carrying no version
   on the left applies to every version, so the `require` still resolves to
   the sibling directory. Assert its absence first:

   ```bash
   (
     cd go
     go mod edit -json | grep -q '"Replace": null' || { echo 'go.mod has a replace'; exit 1; }
     GOWORK=off go test -count=1 ./...
   )
   ```

   `-count=1` because shared fixtures live outside the Go module, so a
   changed corpus does not invalidate the test cache.
3. **Merge the bump through a reviewed PR.** That is the house convention —
   `CONTRIBUTING.md` squash-merges PRs and takes the title as the commit
   message — and what `release.yml`'s own header describes. A direct push to
   `main` is a recovery path, not the normal one: CI still gates it, but
   nothing reviews it, and step 5 then publishes that unreviewed commit
   immutably. If you take it, say so.
4. **Wait for `main` CI to go green on the bump commit.** The release
   workflow **has no test step** — it reads `main`, builds against
   already-published dependencies, publishes and tags. `ci.yml` on the bump
   commit is the only gate there is. An npm version is immutable, and a Go
   module tag is worse: proxy.golang.org caches module versions permanently,
   so a `go/vX.Y.Z` naming the wrong commit cannot be moved, only
   superseded.
5. **Record the release commit, then dispatch.** The confirmation
   below compares each tag against the commit you released, and a run
   that publishes and then fails to tag can be followed by `main`
   moving — so capture it *before* the dispatch, and read it from the
   remote rather than a local ref that may be stale:

   ```bash
   REL=$(git ls-remote origin refs/heads/main | cut -f1)
   ```

   Then dispatch `release.yml` on `main` with `go: true`.

   Keep that SHA. If a later run has to repair this release, the comparison
   must still be against the commit npm actually served — re-reading `main`
   at repair time gives you whatever it has become, which is exactly the
   value the faulty anchor would also produce, so the check would agree with
   itself and pass. If you no longer have it, recover it from the original
   run: the `head_sha` of that `release.yml` run is the commit it published.
6. Confirm — and make the check **fail**, not merely print:

   ```bash
   V=x.y.z
   npm view @tabnas/railroad@$V version
   GH=$(npm view @tabnas/railroad@$V gitHead)
   [ -n "$GH" ] || { echo "npm records no gitHead for $V"; exit 1; }
   for T in "ts/v$V" "go/v$V"; do
     S=$(git ls-remote origin "refs/tags/$T" | cut -f1)
     [ -n "$S" ] || { echo "missing tag $T"; exit 1; }
     [ "$S" = "$GH" ] || { echo "$T is $S, but npm shipped $GH"; exit 1; }
   done
   [ "$GH" = "$REL" ] || { echo "shipped $GH, not the $REL you cleared"; exit 1; }
   ```

   Counting the refs is not enough either. `grep v$V` exits 0 when *either*
   ref matches; a bare `wc -l` prints the count and exits 0 regardless; and
   even `[ "$n" = 2 ]` passes in the case this section warns about, because an
   anchor fallback writes *both* tags on a commit npm never served — and two
   wrong tags count as two. Comparing each tag against the commit you
   released is what catches that.

   The refs carry the commit directly: `release.yml` creates them with
   `git tag "$T" "$ANCHOR"`, so they are lightweight and there is no `^{}`
   to peel.

   `$REL` is deliberately not what the tags are measured against. It is
   your record of what you meant to release, and a repair can make the
   tags agree with it while npm serves something else: publish from A,
   lose the atomic tag push, re-capture `main` at B, and the repair tags
   B — so a `$REL`-only loop passes while the registry still serves A.
   `gitHead` is npm's own record of the commit the tarball was built from,
   so that is what the tags are checked against, and `$REL` is checked
   separately, as the CI question it actually is.

   When the script exits nonzero, the line that failed says what to do. A
   tag that is not `$GH` is wrong, and the two are not equally
   recoverable. A wrong `ts/v$V` simply moves: npm resolves from the
   registry, so the tag is a signpost and nothing reads it. A wrong
   `go/v$V` does not. `proxy.golang.org` caches a module version's content
   immutably, so once anything has fetched `v$V` that content is what
   consumers get for good, and a corrected tag only makes Git and the
   proxy disagree — and you cannot find out whether it has been fetched
   without causing it, because asking the proxy is itself a fetch. Leave
   that tag where it is and release the next patch from the right commit,
   carrying `retract v$V` in its `go/go.mod`: the cached content stays,
   but `go get` stops selecting the bad version and reports it as
   retracted.

   The last line is a different failure. The tags are honest and `$REL` is
   the stale capture — `main` moved before the run checked out — but what
   shipped is then a commit you never cleared CI on, and `release.yml`
   runs no tests of its own. Confirm `$GH` is green on `main` before
   calling the release good.

### When a dispatch dies half-way

The workflow fails closed on a dispatch from any ref but `main`, and when
every tag it would create already exists (the "you forgot to bump" signal).
It fails *open* on an already-published npm version, so a run that published
and then died before tagging can be re-dispatched — **but only while `main`
still points at the release commit.**

That caveat is the sharp edge. The repair logic anchors new tags to an
*existing* tag. If the run published to npm and died before the atomic push,
neither tag exists to supply that anchor — so if `main` has moved on, the
anchor falls back to the new `HEAD` while the publish step skips the version
already on npm. Both tags then land on a commit that is not the one npm
serves, and for the Go module that is permanent. In that state, recover the
original SHA and tag it by hand, or bump to the next patch. Do not just
re-dispatch.

### Never commit the local wiring

Testing against unreleased siblings means symlinked `node_modules`,
`replace` directives and a workspace. None of it may reach a commit, and
`git add -A` is how it does:

- `go mod edit -replace …=/abs/path` — CI reports it as `replacement
  directory /… does not exist`.
- **`go.sum`, after the replace comes out.** A `replace` makes the sibling's
  sums unused, so `go mod tidy` drops them; reverting `go.mod` alone then
  leaves `missing go.sum entry` — a *different* error on the commit meant to
  fix the first one. Revert both, and diff them against the last release
  commit.
- **A `go.work` belongs outside every repo**, one level up. Be precise about
  what it does and does not check: it still consults the `go.sum` files of
  its member modules and writes any missing sums to `go.work.sum`. What it
  skips is validating the *declared version* of a module it replaces with a
  local one — which is exactly the part that hides a bad dependency bump,
  and why the `GOWORK=off` run above exists.
- Scratch files — anything written to measure something.

Stage deliberately (`git add <path>`) and read `git status --short` before
every commit. This bites hardest on a PR whose CI is *expected* red for a
known dependency: a fresh breakage hides inside the expected failure.

### `make publish-ts` and `make publish-go` are not the release path

They predate `release.yml`. Read what each actually does before using
either:

- `publish-ts` runs a local `npm publish`, which goes out over a token and
  bypasses the OIDC trusted publishing the workflow uses.
- `publish-go V=x.y.z` breaks the version invariant: it `sed`s and stages
  **only** `go/model.go`, leaving `ts/package.json` and `VERSION` in
  `ts/src/railroad.ts` on the previous version — the exact state the version
  tests exist to reject. Its `test-go` prerequisite also runs *before* the
  `sed`, so what it verifies is not what it tags.

They stay in the Makefile because removing them is a separate change.

## Agent tooling

An agent working in this repository does not have to drive it by hand. The
org ships two things that already understand these grammars:

- **[`@tabnas/mcp`](https://github.com/tabnas/mcp)** — an MCP server (stdio)
  and the unified `tabnas` CLI: parse, validate and inspect any tabnas
  format, this one included.
- **[`tabnas/skills`](https://github.com/tabnas/skills)** — Agent Skills for
  working on tabnas grammars and plugins.

Prefer them over ad-hoc scripts when exploring a grammar or checking a parse
result.
