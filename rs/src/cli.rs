/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The `tabnas-railroad` command: render railroad (syntax) diagrams from
//! a tabnas grammar. Two modes:
//!
//! - grammar mode (`--grammar <name>`): build a fresh instance with the
//!   named built-in grammar, introspect it, and write
//!   `grammar.railroad.json`, `grammar.svg` and `grammar.txt` into the
//!   output directory (default `./out`).
//! - render mode (`-f <model.json>`, or a bare `-` for stdin): read a
//!   saved [`GrammarModel`] and render one format to stdout.
//!
//! This is the port of `go/cmd/tabnas-railroad`. Like Go, Rust has no
//! dynamic module loading, so grammar mode resolves a fixed table of
//! built-in grammars (currently `json`); render mode is fully general.
//! [`run`] takes its streams as arguments so the tests drive it in
//! process, the seam `run(argv, console)` gives the TypeScript CLI.

use std::io::{Read, Write};
use std::path::Path;

use tabnas::Tabnas;

use crate::{
    extract_grammar, model_to_ascii, model_to_svg, to_text, AsciiOptions, ExtractOptions,
    GrammarModel,
};

#[derive(Debug, Default, Clone)]
struct Args {
    help: bool,
    stdin: bool,
    grammar: Option<String>,
    file: Option<String>,
    out: Option<String>,
    svg: bool,
    ascii: bool,
    json: bool,
    text: bool,
    plain: bool,
}

impl Args {
    fn any_format(&self) -> bool {
        self.svg || self.ascii || self.json || self.text
    }
}

fn parse_args(argv: &[String]) -> Args {
    let mut args = Args::default();
    let mut i = 1;
    while i < argv.len() {
        let arg = argv[i].as_str();
        let take = |i: &mut usize| -> Option<String> {
            *i += 1;
            argv.get(*i).cloned()
        };
        match arg {
            "-" => args.stdin = true,
            "--help" | "-h" => args.help = true,
            "--grammar" | "-g" => args.grammar = take(&mut i),
            "--file" | "-f" => args.file = take(&mut i),
            "--out" | "-o" => args.out = take(&mut i),
            "--svg" => args.svg = true,
            "--ascii" => args.ascii = true,
            "--json" => args.json = true,
            "--text" => args.text = true,
            "--ascii-plain" => {
                args.ascii = true;
                args.plain = true;
            }
            _ => {}
        }
        i += 1;
    }
    args
}

/// Run the command over `argv` (the program name first), reading a
/// model from `stdin` in `-` mode and writing to `stdout` and `stderr`.
/// Returns the process exit code.
pub fn run(
    argv: &[String],
    stdin: &mut dyn Read,
    stdout: &mut dyn Write,
    stderr: &mut dyn Write,
) -> i32 {
    // A write to a closed stdout is the caller's problem, not ours.
    run_inner(argv, stdin, stdout, stderr).unwrap_or(1)
}

fn run_inner(
    argv: &[String],
    stdin: &mut dyn Read,
    stdout: &mut dyn Write,
    stderr: &mut dyn Write,
) -> std::io::Result<i32> {
    let args = parse_args(argv);

    if args.help {
        print_help(stdout)?;
        return Ok(0);
    }

    // Obtain a GrammarModel.
    let model: GrammarModel = if let Some(grammar) = &args.grammar {
        match grammar_from_name(grammar) {
            Ok(model) => model,
            Err(message) => {
                writeln!(stderr, "tabnas-railroad: {}", first_line(&message))?;
                return Ok(1);
            }
        }
    } else if args.file.is_some() || args.stdin {
        let mut src = String::new();
        if let Some(file) = &args.file {
            match std::fs::read_to_string(file) {
                Ok(text) => src = text,
                Err(error) => {
                    writeln!(
                        stderr,
                        "tabnas-railroad: {}",
                        first_line(&error.to_string())
                    )?;
                    return Ok(1);
                }
            }
        }
        if src.trim().is_empty() || args.stdin {
            let mut rest = String::new();
            let _ = stdin.read_to_string(&mut rest);
            src.push_str(&rest);
        }
        match GrammarModel::from_json(&src) {
            Ok(model) => model,
            Err(error) => {
                writeln!(
                    stderr,
                    "tabnas-railroad: invalid JSON grammar model: {}",
                    error.message
                )?;
                return Ok(1);
            }
        }
    } else {
        print_help(stdout)?;
        return Ok(0);
    };

    // Grammar mode (or any mode with -o) writes the three artifacts.
    if args.grammar.is_some() || args.out.is_some() {
        let dir = args.out.clone().unwrap_or_else(|| "out".to_string());
        let mut want = args.clone();
        if !want.any_format() {
            want.svg = true;
            want.ascii = true;
            want.json = true;
        }
        if let Err(error) = std::fs::create_dir_all(&dir) {
            writeln!(stderr, "tabnas-railroad: {error}")?;
            return Ok(1);
        }
        let mut written: Vec<&str> = Vec::new();
        let mut write = |name: &'static str, content: String| -> std::io::Result<bool> {
            match std::fs::write(Path::new(&dir).join(name), content) {
                Ok(()) => Ok(true),
                Err(error) => {
                    writeln!(stderr, "tabnas-railroad: {error}")?;
                    Ok(false)
                }
            }
        };
        if want.json {
            if !write("grammar.railroad.json", model.to_json_pretty())? {
                return Ok(1);
            }
            written.push("grammar.railroad.json");
        }
        if want.svg {
            if !write("grammar.svg", model_to_svg(&model))? {
                return Ok(1);
            }
            written.push("grammar.svg");
        }
        if want.ascii || want.text {
            let out = if want.text {
                text_model(&model)
            } else {
                model_to_ascii(&model, &AsciiOptions { ascii: args.plain })
            };
            if !write("grammar.txt", out)? {
                return Ok(1);
            }
            written.push("grammar.txt");
        }
        writeln!(stdout, "wrote {} to {}/", written.join(", "), dir)?;
        return Ok(0);
    }

    // Render mode: stdout, a single format (default SVG).
    if args.json {
        writeln!(stdout, "{}", model.to_json_pretty())?;
    } else if args.text {
        writeln!(stdout, "{}", text_model(&model))?;
    } else if args.ascii {
        writeln!(
            stdout,
            "{}",
            model_to_ascii(&model, &AsciiOptions { ascii: args.plain })
        )?;
    } else {
        writeln!(stdout, "{}", model_to_svg(&model))?;
    }
    Ok(0)
}

/// Build a fresh instance with a built-in grammar plugin and introspect
/// it. The `#export` suffix the TypeScript CLI accepts is tolerated and
/// ignored: a built-in grammar has one plugin.
fn grammar_from_name(spec: &str) -> Result<GrammarModel, String> {
    let name = spec.split('#').next().unwrap_or(spec);
    let parser: Tabnas = match name {
        "json" | "@tabnas/json" | "tabnas/json" | "tabnas-json" | "tabnas_json" => {
            tabnas_json::make()
        }
        _ => {
            return Err(format!(
                "could not find a grammar plugin for {name:?} (built-in grammars: json)"
            ))
        }
    };
    Ok(extract_grammar(&parser, &ExtractOptions::default()))
}

/// Compact per-rule EBNF-ish text: `name = <node text>`.
pub fn text_model(model: &GrammarModel) -> String {
    model
        .rule_order()
        .into_iter()
        .map(|name| format!("{name} = {}", to_text(&model.rules[name])))
        .collect::<Vec<String>>()
        .join("\n")
}

fn first_line(s: &str) -> &str {
    s.lines().next().unwrap_or("")
}

fn print_help(out: &mut dyn Write) -> std::io::Result<()> {
    out.write_all(HELP.as_bytes())
}

const HELP: &str = "
tabnas-railroad: render railroad (syntax) diagrams from a tabnas grammar.

Usage:
  tabnas-railroad --grammar <name> [-o <dir>] [formats]
  tabnas-railroad -f <model.json> [format]
  echo '<model.json>' | tabnas-railroad - [format]

Modes:
  --grammar <name>         Build a fresh Tabnas instance with the built-in
  -g <name>                  grammar <name>, introspect it, and write
                           artifacts. e.g. --grammar json
                           (built-in grammars: json)
  --file <path>            Render a saved GrammarModel JSON file.
  -f <path>
  -                        Read a GrammarModel JSON from stdin.

Output:
  -o <dir>                 Write grammar.railroad.json + grammar.svg +
                            grammar.txt into <dir> (default ./out in
                            grammar mode). Without -o, render mode prints
                            one format to stdout.

Formats (default: all three when writing, SVG to stdout):
  --json                   Declarative JSON model.
  --svg                    Vertical-flow SVG.
  --ascii                  Vertical ASCII diagram.
  --ascii-plain            ASCII with plain | - + glyphs (implies --ascii).
  --text                   Compact per-rule EBNF text.

  --help, -h               Print this help.

Examples:
  > tabnas-railroad --grammar json -o diagrams
  > tabnas-railroad -f diagrams/grammar.railroad.json --ascii
  > tabnas-railroad --grammar json --text -o /tmp/rr
";
