/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The `tabnas-railroad` command. The implementation is
//! `tabnas_railroad::cli::run`, which takes its streams as arguments so
//! the tests can drive it in process; this is the launcher.

use std::io::IsTerminal;

fn main() {
    let argv: Vec<String> = std::env::args().collect();
    // A terminal on stdin holds no model: the TypeScript command reads
    // nothing from a TTY, so an empty `-f` file (or a bare `-`) fails
    // with an invalid-model error instead of waiting for a keyboard.
    let mut stdin = std::io::stdin();
    let mut nothing = std::io::empty();
    let input: &mut dyn std::io::Read = if stdin.is_terminal() {
        &mut nothing
    } else {
        &mut stdin
    };
    let code = tabnas_railroad::cli::run(
        &argv,
        input,
        &mut std::io::stdout().lock(),
        &mut std::io::stderr().lock(),
    );
    std::process::exit(code);
}
