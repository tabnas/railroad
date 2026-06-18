# github.com/tabnas/railroad/go

Go port of [`@tabnas/railroad`](../ts) — a railroad (syntax) diagram
generator for the [`tabnas`](https://github.com/tabnas/parser) parsing
engine.

It does **not** parse anything itself: it **introspects a live `Tabnas`
instance that already has a grammar installed** and emits three artifacts
from that grammar —

- a declarative, JSON-serializable **`GrammarModel`** (the interchange
  format: one node tree per rule),
- a vertical-flow **SVG** (one anchored, linked track per rule), and
- a vertical **ASCII** diagram (Unicode box-drawing, or plain `| - +`).

It also ships the **`tabnas-railroad` CLI**. Diagrams bias toward
verticality (tall and narrow) so they read on laptops and phones:
sequences run top-to-bottom, choices fan out sideways, optional /
repetition rails run on the side.

The Go package tracks the canonical TypeScript implementation in `../ts`:
the extracted `GrammarModel` JSON matches the TS model for the same grammar
(same `start` / rules / structure), and the SVG/ASCII renderers produce the
same vertical-flow output.

## Documentation

Four-quadrant [Diátaxis](https://diataxis.fr/) docs:

- [doc/tutorial.md](doc/tutorial.md) — zero to a rendered diagram, step by step.
- [doc/guide.md](doc/guide.md) — focused how-to recipes.
- [doc/reference.md](doc/reference.md) — the exact API surface and CLI flags.
- [doc/concepts.md](doc/concepts.md) — how introspection and vertical-flow
  layout work, and the [differences from the TS version](doc/concepts.md#differences-from-the-ts-version).

The canonical TypeScript implementation lives in [`../ts`](../ts).

## Install

```bash
go get github.com/tabnas/railroad/go
```

## Use

```go
package main

import (
	"fmt"

	jsonplugin "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
	tabnasrailroad "github.com/tabnas/railroad/go"
)

func main() {
	tn := tabnas.Make()
	jsonplugin.Json(tn, nil) // install a grammar to diagram
	tabnasrailroad.Plugin(tn, nil) // decorate the instance

	api := tabnasrailroad.Of(tn)
	model := api.ToJson()    // *GrammarModel
	svg, _ := api.ToSvg()    // whole-grammar SVG
	ascii, _ := api.ToAscii(tabnasrailroad.AsciiOptions{}) // whole-grammar ASCII

	fmt.Println(model.Start) // val
	_ = svg
	_ = ascii
}
```

Instance-free use of the extractor and renderers:

```go
model := tabnasrailroad.ExtractGrammar(tn)
svg, _ := tabnasrailroad.ModelToSvg(model)
ascii, _ := tabnasrailroad.ModelToAscii(model, tabnasrailroad.AsciiOptions{Plain: true})
text, _ := tabnasrailroad.ToText(model.Rules["val"])
```

Hand-build a diagram node tree and render a single node:

```go
node := tabnasrailroad.Diagram(tabnasrailroad.Sequence(
	tabnasrailroad.Terminal("GET"), tabnasrailroad.NonTerminal("path")))
svg, _ := tabnasrailroad.RenderNodeSvg(node)
```

## Exports

- `ExtractGrammar(tn, *ExtractOptions) *GrammarModel` — introspect a live
  instance into the model. **The heart of the package.**
- `ModelToSvg`, `ModelToAscii`, `RenderNodeSvg`, `RenderNodeAscii`,
  `ToText` — renderers.
- Node constructors: `Terminal`, `NonTerminal`, `Comment`, `SkipNode`,
  `Sequence`, `Choice`, `MustChoice`, `Optional`, `OneOrMore`,
  `ZeroOrMore`, `Diagram`.
- `Plugin(tn, opts)` + `Of(tn)` — the plugin wiring and API accessor.
- Types: `RailroadNode`, `GrammarModel`, `LegendEntry`, `RailroadError`.

## Notes on the Go port

The TypeScript extractor reads the raw `#KEY` / `#VAL` token-set names off
each alt to render readable set labels. The Go engine resolves those names
to `[]Tin` sets on the live `RuleSpec`, so the Go extractor recovers the
set name by matching the resolved tins to a named token set (disambiguating
identical sets — e.g. `KEY` vs `VAL` — by position role: a slot followed by
a colon is a map key). Rule-map key order is not part of the cross-language
contract; the Go model orders user rules deterministically (the engine's
`RSM` is an unordered map).

## Sample output

The json grammar rendered to a vertical-flow diagram
(`go run ./cmd/tabnas-railroad --grammar json -o examples`):

![railroad diagram of the json grammar](../examples/json-grammar.svg)

The same grammar as an ASCII diagram (excerpt — the `val` choice and the
`pair` loop; full output in
[`../examples/json-grammar.txt`](../examples/json-grammar.txt)):

```text
val:
              │
   ┌──────────┼──────────┐
┌──┴──┐   ┌───┴──┐   ╭───┴───╮
│ map │   │ list │   │ "VAL" │
└──┬──┘   └───┬──┘   ╰───┬───╯
   └──────────┼──────────┘
              │

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

## CLI

```bash
go run ./cmd/tabnas-railroad --grammar json -o diagrams      # write all three
go run ./cmd/tabnas-railroad -f diagrams/grammar.railroad.json --ascii
go run ./cmd/tabnas-railroad --grammar json --text -o /tmp/rr
```

## License

MIT.
