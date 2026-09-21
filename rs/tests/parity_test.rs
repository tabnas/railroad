/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Cross-runtime conformance, driven by the shared `test/spec/*.tsv`
//! fixtures at the repo root (see `../test/AGENTS.md`).
//!
//! The fixture loader, the escape codec, the `ERROR:` contract and the
//! row loop all come from `tabnas_support`, the Rust half of
//! `@tabnas/support`, whose TypeScript and Go halves run the SAME files
//! in `ts/test/parity.test.js` and `go/parity_test.go`. So the renderers
//! cannot drift without one runtime going red, and neither can the
//! loaders.
//!
//! What is left here is only what is specific to railroad: which
//! renderer a fixture is for.

mod common;

use std::path::Path;

use tabnas_railroad::{render_node_ascii, to_text, AsciiOptions, RailroadNode};
use tabnas_support::{find_spec_dir, load_spec_dir, Failure, Row, Runner, SpecOptions, Value};

/// The renderers a fixture's second column can name.
const RENDERERS: &[&str] = &["text", "ascii"];

/// Every fixture in the spec directory. The second column's HEADER names
/// the renderer, which is why there is a runner per file rather than one
/// over the directory.
#[test]
fn spec() {
    let dir = find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR")))).expect("test/spec");
    let specs = load_spec_dir(&dir, &SpecOptions::default()).expect("the fixtures load");
    assert!(!specs.is_empty(), "{}: no fixtures", dir.display());

    for spec in &specs {
        assert!(
            spec.header.len() >= 2,
            "{}: expected at least two columns",
            spec.file
        );
        let kind = spec.header[1].clone();
        assert!(
            RENDERERS.contains(&kind.as_str()),
            "{}: unknown second column {kind:?}",
            spec.file
        );

        let renderer = kind.clone();
        Runner::new_with_row(move |input, row| render_spec(&renderer, input, row))
            // The rendered text is compared against the expected column,
            // which holds it as a JSON string, so the comparison is the
            // runner's ordinary one, over two strings.
            .expected(kind.as_str())
            .spec(spec);
    }
}

/// Every fixture file is run: a file that stopped being discovered would
/// otherwise pass silently. The check is that the baseline files are
/// present, not that they are the only ones, so a fixture added to the
/// shared directory is picked up here without touching this runner.
#[test]
fn every_fixture_is_discovered() {
    let dir = find_spec_dir(Some(Path::new(env!("CARGO_MANIFEST_DIR")))).expect("test/spec");
    let specs = load_spec_dir(&dir, &SpecOptions::default()).expect("the fixtures load");
    let names: Vec<&str> = specs.iter().map(|spec| spec.file.as_str()).collect();
    for required in ["node-ascii.tsv", "node-text.tsv"] {
        assert!(
            names.contains(&required),
            "{required} is not discovered: {names:?}"
        );
    }
}

/// The first column is the node's own JSON shape, which is what both
/// runtimes marshal to and unmarshal from. The third is renderer
/// options, when the renderer takes any.
fn render_spec(kind: &str, input: &str, row: &Row) -> Result<Value, Failure> {
    let node = RailroadNode::from_json(input).map_err(|error| Failure::message(error.message))?;

    let opts_cell = row.named("opts");
    let opts: serde_json::Value = if opts_cell.trim().is_empty() {
        serde_json::Value::Null
    } else {
        serde_json::from_str(opts_cell)
            .map_err(|error| Failure::message(format!("opts column: {error}")))?
    };

    let rendered = match kind {
        "text" => to_text(&node),
        "ascii" => {
            let plain = opts
                .get("ascii")
                .and_then(serde_json::Value::as_bool)
                .unwrap_or(false);
            render_node_ascii(&node, &AsciiOptions { ascii: plain })
        }
        other => return Err(Failure::message(format!("unknown renderer {other:?}"))),
    };
    Ok(Value::String(rendered))
}
