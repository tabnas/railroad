/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Shared test scaffolding.
//!
//! Cargo compiles this module into EVERY integration test binary, so a
//! helper only one of them uses reads as dead code in the others. The
//! allow is about that compilation model, not about unused code.

#![allow(dead_code)]

use std::path::{Path, PathBuf};

use tabnas::Tabnas;
use tabnas_railroad::RailroadApi;

/// The repository root: `rs/`'s parent.
pub fn repo_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
        .to_path_buf()
}

/// A `Tabnas` instance with the json grammar and the railroad plugin
/// loaded: the known-good fixture grammar, the role `@tabnas/json` plays
/// in `ts/test/grammar.test.js` and `go/grammar_test.go`.
pub fn build() -> Tabnas {
    let mut parser = tabnas_json::make();
    tabnas_railroad::railroad(&mut parser).expect("the railroad plugin installs");
    parser
}

/// The railroad API bound to `parser`.
pub fn api(parser: &Tabnas) -> RailroadApi<'_> {
    tabnas_railroad::of(parser)
}

/// An integer attribute value read out of SVG text.
pub fn svg_attr(svg: &str, name: &str) -> i64 {
    let key = format!("{name}=\"");
    let at = svg
        .find(&key)
        .unwrap_or_else(|| panic!("attribute {name:?} not found"))
        + key.len();
    let digits: String = svg[at..].chars().take_while(char::is_ascii_digit).collect();
    digits.parse().expect("an integer attribute")
}
