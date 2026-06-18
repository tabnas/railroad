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
	railroad "github.com/tabnas/railroad/go"
)

func main() {
	tn := tabnas.Make()
	jsonplugin.Json(tn, nil) // install a grammar to diagram
	railroad.Plugin(tn, nil) // decorate the instance

	api := railroad.Of(tn)
	model := api.ToJson()    // *GrammarModel
	svg, _ := api.ToSvg()    // whole-grammar SVG
	ascii, _ := api.ToAscii(railroad.AsciiOptions{}) // whole-grammar ASCII

	fmt.Println(model.Start) // val
	_ = svg
	_ = ascii
}
```

Instance-free use of the extractor and renderers:

```go
model := railroad.ExtractGrammar(tn)
svg, _ := railroad.ModelToSvg(model)
ascii, _ := railroad.ModelToAscii(model, railroad.AsciiOptions{Plain: true})
text, _ := railroad.ToText(model.Rules["val"])
```

Hand-build a diagram node tree and render a single node:

```go
node := railroad.Diagram(railroad.Sequence(
	railroad.Terminal("GET"), railroad.NonTerminal("path")))
svg, _ := railroad.RenderNodeSvg(node)
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

## CLI

```bash
go run ./cmd/tabnas-railroad --grammar json -o diagrams      # write all three
go run ./cmd/tabnas-railroad -f diagrams/grammar.railroad.json --ascii
go run ./cmd/tabnas-railroad --grammar json --text -o /tmp/rr
```

## License

MIT.
