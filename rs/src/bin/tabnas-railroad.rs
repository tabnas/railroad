/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The `tabnas-railroad` command. The implementation is
//! `tabnas_railroad::cli::run`, which takes its streams as arguments so
//! the tests can drive it in process; this is the launcher.

fn main() {
    let argv: Vec<String> = std::env::args().collect();
    let code = tabnas_railroad::cli::run(
        &argv,
        &mut std::io::stdin().lock(),
        &mut std::io::stdout().lock(),
        &mut std::io::stderr().lock(),
    );
    std::process::exit(code);
}
