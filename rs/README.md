# tabnas-railroad (Rust)

Railroad (syntax) diagram renderer for the
[`tabnas`](https://github.com/tabnas/parser) parser, crate
`tabnas-railroad` (library `tabnas_railroad`).

It does not parse anything itself. It introspects a live `Tabnas`
instance that already has a grammar installed and emits three artifacts
from that grammar:

- a declarative, JSON-serializable `GrammarModel` (the interchange
  format: one node tree per rule),
- a vertical-flow SVG (one anchored, linked track per rule), and
- a vertical ASCII diagram (Unicode box drawing, or plain `| - +`).

It also ships the `tabnas-railroad` command. Diagrams bias toward
verticality (tall and narrow) so they read on laptops and phones:
sequences run top to bottom, choices fan out sideways, optional and
repetition rails run on the side.

This is the Rust port of the canonical TypeScript implementation in
[`../ts`](../ts); the TypeScript version is authoritative and this crate
tracks it. The same model gives the same bytes: the suite holds the
extracted model of the json grammar to the TypeScript snapshot, and the
SVG and ASCII renderings to the TypeScript-generated files in
[`../examples`](../examples).

## Use

```rust
use tabnas_railroad::{extract_grammar, model_to_ascii, model_to_svg, AsciiOptions, ExtractOptions};

fn main() {
    let parser = tabnas_json::make();
    let model = extract_grammar(&parser, &ExtractOptions::default());

    assert_eq!(model.start, "val");
    assert_eq!(model.rules.len(), 5);

    let svg = model_to_svg(&model);
    let ascii = model_to_ascii(&model, &AsciiOptions::default());
    let json = model.to_json_pretty();
    assert!(svg.starts_with("<svg "));
    assert!(ascii.starts_with("val:\n"));
    assert!(json.starts_with("{\n  \"start\": \"val\""));
}
```

The API can also be bound to an instance, the shape `tn.railroad` takes
in TypeScript:

```rust
fn main() {
    let parser = tabnas_json::make();
    let api = tabnas_railroad::of(&parser);
    let model = api.to_json();
    let svg = api.to_svg();
    let ascii = api.to_ascii(&tabnas_railroad::AsciiOptions::plain());
    assert_eq!(model.start, "val");
    assert!(svg.starts_with("<svg "));
    assert!(ascii.is_ascii());
}
```

Installing the plugin marks the instance and nothing more: every helper
re-reads the instance's current grammar when called, so plugin install
order does not matter.

```rust
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut parser = tabnas_json::make();
    tabnas_railroad::railroad(&mut parser)?;
    assert!(tabnas_railroad::installed(&parser));
    Ok(())
}
```

Diagrams can be built by hand, without an instance. A bare string is a
terminal:

```rust
use tabnas_railroad::{diagram, non_terminal, optional, sequence, to_text, zero_or_more};

fn main() {
    let list = diagram([sequence([
        "[".into(),
        optional(sequence([
            non_terminal("item"),
            zero_or_more(sequence([",".into(), non_terminal("item")]), None),
        ])),
        "]".into(),
    ])]);
    assert_eq!(to_text(&list), r#""[" [item {"," item}] "]""#);
}
```

`render_node_svg` and `render_node_ascii` render one node; `to_text`
gives the compact EBNF-ish form shown above.

### The command

```bash
tabnas-railroad --grammar json -o diagrams          # writes grammar.railroad.json, grammar.svg, grammar.txt
tabnas-railroad -f diagrams/grammar.railroad.json --ascii
cat diagrams/grammar.railroad.json | tabnas-railroad - --text
```

Grammar mode resolves a fixed table of built-in grammars (`json`), as
the Go command does: Rust has no dynamic module loading. Render mode
takes any saved model. `tabnas-railroad -h` lists the flags.

## Install

None of the tabnas crates is published to a registry, so the engine is
consumed as a **sibling checkout**, the standard tabnas development
model. Clone `https://github.com/tabnas/parser` next to this repository
and point at it:

```toml
[dependencies]
tabnas = { path = "../parser/rs" }
tabnas-railroad = { path = "../railroad/rs" }
```

Both entries are needed: a crate's dependencies are not passed on to its
dependents, so `tabnas-railroad` alone does not put `tabnas` in your
extern prelude.

The `cli` feature, on by default, builds the command and takes the
`tabnas-json` crate from `../json/rs` for its built-in grammar. A library
consumer that wants neither takes the crate with
`default-features = false`. The test suite needs `../json/rs` and
`../support/rs` as siblings too.

## Differences from the canonical TypeScript

The model, the SVG and the ASCII are the same. The differences are in
how the crate reaches the engine and in what Rust can express:

- **Named token sets are recovered, not read.** The TypeScript extractor
  reads the raw `#KEY` / `#VAL` set name straight off the alt spec. This
  engine resolves a spec to token numbers and keeps no raw names, so a
  set name is recovered by matching a position's tokens against the
  instance's named sets, preferring `KEY` for a position followed by a
  colon. This is the Go port's approach, and it renders the json grammar
  identically. One consequence: a position that names a single token
  whose one-member set exists (a bare `#ST` where `KEY` is `["#ST"]`)
  renders as the set name, where TypeScript would show the token, and
  only when the set's name starts with an ASCII letter, the same gate
  TypeScript puts on a raw set name. `ExtractOptions::token_set_names`
  narrows the candidates.
- **Token descriptions are an option, not a config hook.** TypeScript
  reads `cfg.tokenDesc`, which a grammar attaches through the engine's
  `config.modify` hook. This engine has no such bag, so descriptions are
  passed in `ExtractOptions::token_desc`, keyed by bare or `#`-prefixed
  name, as the Go port takes them.
- **Errors are typed, so they arrive earlier.** A node is an enum: it
  cannot hold an unknown `kind`, so `{"kind":"bogus"}` is refused when
  the JSON is decoded (`RailroadNode::from_json`), not when it reaches a
  renderer, and the renderers and `to_text` are infallible. `choice`
  with no branches is the one constructor that fails, as it is in
  TypeScript.
- **The plugin is a mark, and the API is bound on demand.** TypeScript
  decorates the instance with a callable. A Rust decoration cannot hold
  a reference to the instance it sits on, so the plugin leaves a unit
  mark and `of(&parser)` binds the API to any instance, installed or
  not.
- **The command's grammar mode is a table.** `--grammar json` (and the
  spellings `@tabnas/json`, `tabnas-json`) builds the json grammar;
  there is no module loading. Render mode is fully general.

## Build and test

The engine, the json grammar and the support crate are path dependencies
on sibling checkouts, so there is nothing to fetch:

```bash
cargo test --all-targets
```

Or, from the repository root, `make test-rs`. For what CI would say,
including formatting, doctests and the lockfile check, run
`ci/rust/run.sh`.

The suite runs the shared `../test/spec/*.tsv` fixtures, the same files
the TypeScript and Go suites run, through the `tabnas-support` runner;
compares the extracted model of the json grammar with the TypeScript
snapshot in `../go/testdata`, by value and by bytes; and holds the
whole-grammar SVG and ASCII to the files in `../examples`.

## License

MIT.
