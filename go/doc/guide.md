# How-to guide — railroad recipes (Go)

Focused recipes for the Go port `tabnasrailroad`. Each is self-contained.
For the full API see [reference.md](reference.md); for the why see
[concepts.md](concepts.md).

```bash
go get github.com/tabnas/railroad/go
```

All recipes import:

```go
import (
	jsonplugin "github.com/tabnas/json/go"
	tabnas "github.com/tabnas/parser/go"
	tabnasrailroad "github.com/tabnas/railroad/go"
)
```

## Diagram your own grammar

`tabnasrailroad` draws whatever grammar is installed on the instance. Swap
the json plugin for your own grammar plugin:

```go
tn := tabnas.Make()
myGrammar(tn, nil)              // your grammar plugin
tabnasrailroad.Plugin(tn, nil)

ascii, _ := tabnasrailroad.Of(tn).ToAscii(tabnasrailroad.AsciiOptions{})
fmt.Println(ascii)
```

Install order is irrelevant — the helpers re-introspect the live grammar on
each call.

## Save the three artifacts to disk

```go
tn := tabnas.Make()
jsonplugin.Json(tn, nil)
tabnasrailroad.Plugin(tn, nil)
api := tabnasrailroad.Of(tn)

model := api.ToJson()
jsonBytes, _ := json.MarshalIndent(model, "", "  ")
os.WriteFile("grammar.railroad.json", jsonBytes, 0o644)

svg, _ := api.ToSvg()
os.WriteFile("grammar.svg", []byte(svg), 0o644)

ascii, _ := api.ToAscii(tabnasrailroad.AsciiOptions{})
os.WriteFile("grammar.txt", []byte(ascii), 0o644)
```

Or let the CLI do it in one call (writes exactly these three files):

```bash
go run ./cmd/tabnas-railroad --grammar json -o diagrams
```

## Get plain ASCII (no Unicode box-drawing)

Set `Plain: true` (CLI: `--ascii-plain`). Output is pure 7-bit ASCII using
`| - +`.

```go
plain, _ := api.ToAscii(tabnasrailroad.AsciiOptions{Plain: true})
// plain contains only bytes <= 127
```

## Get a compact text (EBNF-ish) summary

Use `ToText` on a rule node. Terminals are quoted, choice is `(a | b)`,
optional is `[x]`, zero-or-more is `{x}`, one-or-more is `x+`.

```go
model := api.ToJson()

mapText, _ := tabnasrailroad.ToText(model.Rules["map"])
fmt.Println(mapText) // "{" [pair] "}"

valText, _ := tabnasrailroad.ToText(model.Rules["val"])
fmt.Println(valText) // (map | list | "VAL")

pairText, _ := tabnasrailroad.ToText(model.Rules["pair"])
fmt.Println(pairText) // "KEY" ":" val+ /* "," */
```

The CLI exposes the same as `--text`:

```bash
go run ./cmd/tabnas-railroad --grammar json --text -o /tmp/rr
```

## Save the model, render it later (decoupling)

The JSON model is the interchange format: SVG and ASCII are fully
reproducible from it, with no live instance needed. The model round-trips
through `encoding/json` to identical output.

```go
model := api.ToJson()
data, _ := json.Marshal(model)

var clone tabnasrailroad.GrammarModel
json.Unmarshal(data, &clone)

svg1, _ := tabnasrailroad.ModelToSvg(model)
svg2, _ := tabnasrailroad.ModelToSvg(&clone)
// svg1 == svg2
```

`UnmarshalJSON` recovers the rule key order from the raw JSON, so a
round-tripped model renders identically.

With the CLI, write the model once and re-render any time:

```bash
go run ./cmd/tabnas-railroad --grammar json --json -o diagrams
go run ./cmd/tabnas-railroad -f diagrams/grammar.railroad.json --ascii
go run ./cmd/tabnas-railroad -f diagrams/grammar.railroad.json --svg > g.svg
cat diagrams/grammar.railroad.json | go run ./cmd/tabnas-railroad - --text
```

## Hand-build a diagram (no grammar at all)

The node constructors are exported, so you can draw any railroad diagram by
hand. Unlike the TS version, Go terminals are always explicit (there is no
bare-string coercion).

```go
node := tabnasrailroad.Sequence(
	tabnasrailroad.Terminal("GET"),
	tabnasrailroad.Optional(tabnasrailroad.NonTerminal("path")),
)
text, _ := tabnasrailroad.ToText(node)
fmt.Println(text) // "GET" [path]
```

Render a single hand-built node to its own standalone SVG or ASCII block:

```go
node := tabnasrailroad.Diagram(tabnasrailroad.Sequence(
	tabnasrailroad.Terminal("GET"), tabnasrailroad.NonTerminal("path")))
svg, _ := tabnasrailroad.RenderNodeSvg(node)
// svg starts with "<svg "
```

`Choice` returns an `error` on zero branches; `MustChoice` panics instead
(use it where the branch count is guaranteed).

```go
c, err := tabnasrailroad.Choice(tabnasrailroad.Terminal("a"))
_ = c
_ = err
mustC := tabnasrailroad.MustChoice(tabnasrailroad.Terminal("a"))
_ = mustC
```

## Read the token legend

Token labels that are not self-explanatory punctuation (e.g. `KEY` / `VAL`)
get a `Legend` entry; tokens the lexer silently skips (whitespace, comments)
are reported in `Ignored`. Both are also rendered into the SVG and ASCII as
"Tokens" / "Ignored tokens" keys.

```go
model := api.ToJson()

for _, e := range model.Legend {
	fmt.Printf("%s = %s\n", e.Token, e.Meaning)
}
// KEY = map key: bare text, number, string, or keyword
// VAL = value: bare text, number, string, or keyword

for _, e := range model.Ignored {
	fmt.Printf("%s = %s\n", e.Token, e.Meaning)
}
// CM = comment
// LN = newline (line break)
// SP = whitespace (spaces or tabs)
```

## Skip the normalization pass

By default extraction factors common prefix/suffix across choice branches
and turns an empty branch into `optional`. Pass `NoFactor: true` to get the
raw reverse-mapped tree:

```go
raw := api.Extract(&tabnasrailroad.ExtractOptions{NoFactor: true})
_ = raw
```
