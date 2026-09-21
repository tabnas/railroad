/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The port of `go/cmd/tabnas-railroad/main_test.go` (itself the port of
//! the `cli` block in `ts/test/grammar.test.js`): grammar mode writes the
//! three artifacts, render mode renders a saved model from `-f` and from
//! stdin, and `-h` prints help.

#![cfg(feature = "cli")]

mod common;

use std::fs;
use std::path::PathBuf;

use tabnas_railroad::cli::run;

fn args(list: &[&str]) -> Vec<String> {
    std::iter::once("tabnas-railroad")
        .chain(list.iter().copied())
        .map(String::from)
        .collect()
}

fn run_cli(list: &[&str], stdin: &str) -> (i32, String, String) {
    let mut input = stdin.as_bytes();
    let mut stdout: Vec<u8> = Vec::new();
    let mut stderr: Vec<u8> = Vec::new();
    let code = run(&args(list), &mut input, &mut stdout, &mut stderr);
    (
        code,
        String::from_utf8(stdout).expect("utf-8 stdout"),
        String::from_utf8(stderr).expect("utf-8 stderr"),
    )
}

/// A fresh scratch directory under the target directory, removed by the
/// caller's `Scratch` guard.
struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Self {
        let dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("target")
            .join("cli-test")
            .join(format!("{name}-{}", std::process::id()));
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).expect("the scratch directory is creatable");
        Scratch(dir)
    }

    fn path(&self) -> &str {
        self.0.to_str().expect("a utf-8 path")
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn grammar_mode_writes_three_artifacts() {
    let dir = Scratch::new("grammar");
    let (code, stdout, stderr) = run_cli(&["--grammar", "json", "-o", dir.path()], "");
    assert_eq!(code, 0, "stderr: {stderr}");
    assert!(stderr.is_empty(), "unexpected stderr: {stderr}");
    assert_eq!(
        stdout.trim_end(),
        format!(
            "wrote grammar.railroad.json, grammar.svg, grammar.txt to {}/",
            dir.path()
        )
    );

    let json = fs::read_to_string(dir.0.join("grammar.railroad.json")).unwrap();
    let model: serde_json::Value = serde_json::from_str(&json).unwrap();
    assert_eq!(model["start"], "val");

    let svg = fs::read_to_string(dir.0.join("grammar.svg")).unwrap();
    assert!(
        svg.starts_with("<svg ") && svg.ends_with("</svg>"),
        "svg not well-formed"
    );

    let txt = fs::read_to_string(dir.0.join("grammar.txt")).unwrap();
    assert!(txt.contains("val:"), "grammar.txt should contain 'val:'");
}

#[test]
fn grammar_mode_writes_only_the_asked_formats() {
    let dir = Scratch::new("formats");
    let (code, stdout, _) = run_cli(
        &["--grammar", "@tabnas/json", "--text", "-o", dir.path()],
        "",
    );
    assert_eq!(code, 0);
    assert!(stdout.starts_with("wrote grammar.txt to "));
    assert!(!dir.0.join("grammar.svg").exists());
    let txt = fs::read_to_string(dir.0.join("grammar.txt")).unwrap();
    assert!(txt.starts_with("val = "));
}

#[test]
fn render_mode_from_file() {
    let dir = Scratch::new("render");
    let (code, _, stderr) = run_cli(&["--grammar", "json", "-o", dir.path()], "");
    assert_eq!(code, 0, "setup failed: {stderr}");
    let file = dir.0.join("grammar.railroad.json");

    let (code, stdout, stderr) = run_cli(&["-f", file.to_str().unwrap(), "--text"], "");
    assert_eq!(code, 0, "stderr: {stderr}");
    assert!(stdout.starts_with("val = "), "got:\n{stdout}");

    let (code, stdout, _) = run_cli(&["-f", file.to_str().unwrap(), "--ascii-plain"], "");
    assert_eq!(code, 0);
    assert!(stdout.is_ascii() && stdout.contains("val:"));

    let (code, stdout, _) = run_cli(&["-f", file.to_str().unwrap()], "");
    assert_eq!(code, 0);
    assert!(stdout.starts_with("<svg "));

    let (code, stdout, _) = run_cli(&["-f", file.to_str().unwrap(), "--json"], "");
    assert_eq!(code, 0);
    assert_eq!(
        stdout.trim_end(),
        fs::read_to_string(&file).unwrap().trim_end()
    );
}

#[test]
fn render_mode_from_stdin() {
    let parser = common::build();
    let model = common::api(&parser).to_json().to_json();
    let (code, stdout, stderr) = run_cli(&["-", "--text"], &model);
    assert_eq!(code, 0, "stderr: {stderr}");
    assert!(stdout.starts_with("val = "), "got:\n{stdout}");
}

#[test]
fn invalid_model_json_fails() {
    let (code, stdout, stderr) = run_cli(&["-"], "{not json");
    assert_eq!(code, 1);
    assert!(stdout.is_empty());
    assert!(stderr.starts_with("tabnas-railroad: invalid JSON grammar model:"));
}

#[test]
fn empty_choice_in_a_model_fails_instead_of_panicking() {
    let model = r#"{"start":"a","rules":{"a":{"kind":"choice","items":[]}}}"#;
    for format in ["--svg", "--ascii", "--text", "--json"] {
        let (code, stdout, stderr) = run_cli(&["-", format], model);
        assert_eq!(code, 1, "{format}");
        assert!(stdout.is_empty(), "{format}");
        assert!(
            stderr.contains("choice needs at least one branch"),
            "{format}: {stderr}"
        );
    }
}

#[test]
fn unknown_grammar_fails() {
    let (code, _, stderr) = run_cli(&["--grammar", "yaml"], "");
    assert_eq!(code, 1);
    assert!(stderr.contains("could not find a grammar plugin for \"yaml\""));
}

#[test]
fn help() {
    let (code, stdout, _) = run_cli(&["-h"], "");
    assert_eq!(code, 0);
    assert!(stdout.contains("Usage:"));
    // No arguments at all prints help too.
    let (code, stdout, _) = run_cli(&[], "");
    assert_eq!(code, 0);
    assert!(stdout.contains("Usage:"));
}
