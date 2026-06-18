# Tutorial — your first railroad diagram (Go)

This walks you from nothing to a rendered syntax (railroad) diagram of a
real grammar, using the Go port `tabnasrailroad`. One happy path, every step
grounded in the package's tests. By the end you will have a JSON model, an
SVG, and an ASCII diagram of the json grammar.

`tabnasrailroad` never parses anything. It **introspects a live
[`tabnas`](https://github.com/tabnas/parser) parser instance that already
has a grammar installed** and draws that grammar.

> The TypeScript implementation in [`../../ts`](../../ts) is canonical; this
> Go port tracks it. For the same grammar the two produce the same model and
> the same SVG/ASCII. See [concepts.md](concepts.md) for the differences.

## 1. Add the modules

```bash
go get github.com/tabnas/railroad/go
go get github.com/tabnas/json/go
go get github.com/tabnas/parser/go
```

- `github.com/tabnas/parser/go` is the parsing engine.
- `github.com/tabnas/railroad/go` is this package (`package tabnasrailroad`).
- `github.com/tabnas/json/go` is a grammar to draw.

## 2. Build an instance, install a grammar and the plugin

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
	jsonplugin.Json(tn, nil)        // install a grammar to diagram
	tabnasrailroad.Plugin(tn, nil)  // decorate the instance

	api := tabnasrailroad.Of(tn)    // *RailroadApi
	fmt.Println(api != nil)         // true
}
```

`Plugin` decorates the instance under the key `tabnasrailroad.DecorationName`;
`Of(tn)` returns the installed `*RailroadApi`. (`Of` also works without
`Plugin` — it binds a fresh API to the instance — so the plugin call is
optional when you only use `Of`.)

## 3. Read the grammar back out as a model

`api.ToJson()` introspects the installed rules and returns a plain,
JSON-serializable `*GrammarModel`.

```go
api := tabnasrailroad.Of(tn)
model := api.ToJson()

fmt.Println(model.Start)        // val
fmt.Println(len(model.Rules))   // 5
fmt.Println(model.Meta["engine"]) // tabnas
```

The model has a `Start` rule (the entry point), a `Rules` map (one diagram
node per rule), and `Meta`. The five json rules are `val`, `map`, `list`,
`pair`, `elem`.

## 4. Render an ASCII diagram

`api.ToAscii` returns a vertical-flow ASCII diagram — one block per rule. It
takes an `AsciiOptions` (use the zero value for Unicode box-drawing).

```go
ascii, _ := api.ToAscii(tabnasrailroad.AsciiOptions{})
fmt.Println(ascii)
```

The `val` rule renders as a three-way choice, read top to bottom:

```text
val:
              │
   ┌──────────┼──────────┐
┌──┴──┐   ┌───┴──┐   ╭───┴───╮
│ map │   │ list │   │ "VAL" │
└──┬──┘   └───┬──┘   ╰───┬───╯
   └──────────┼──────────┘
              │
```

Square-cornered boxes are **nonterminals** (rule references — `map`,
`list`); round-cornered quoted boxes are **terminals** (tokens — `"VAL"`).
A json value is a map, a list, or a value token.

For terminals that mangle box-drawing characters, ask for plain mode — pure
7-bit ASCII using `| - +`:

```go
plain, _ := api.ToAscii(tabnasrailroad.AsciiOptions{Plain: true})
fmt.Println(plain)
```

## 5. Render an SVG

`api.ToSvg` returns a standalone SVG string: a vertical stack of titled,
anchored rule tracks where nonterminal boxes link to the referenced rule.

```go
svg, _ := api.ToSvg()
fmt.Println(svg[:5] == "<svg ")          // true
fmt.Println(svg[len(svg)-6:] == "</svg>") // true
```

Write it to a file and open it in a browser:

```go
import "os"

svg, _ := api.ToSvg()
os.WriteFile("json-grammar.svg", []byte(svg), 0o644)
```

## 6. Do it all from the command line

The package ships a CLI under `cmd/tabnas-railroad`:

```bash
# write grammar.railroad.json + grammar.svg + grammar.txt into ./diagrams
go run ./cmd/tabnas-railroad --grammar json -o diagrams
```

That introspects the json grammar on a fresh instance and writes all three
artifacts. (Go has no dynamic module loading, so `--grammar` resolves a
fixed set of built-in grammar names; `json` is built in.)

## Where to go next

- [guide.md](guide.md) — focused recipes.
- [reference.md](reference.md) — every export, option, and CLI flag.
- [concepts.md](concepts.md) — how introspection and vertical-flow layout
  work, plus the differences from the TS version.
