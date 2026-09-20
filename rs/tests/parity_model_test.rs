/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The cross-language contract on the whole extracted model, the port of
//! `go/parity_model_test.go`: this port's model of the json grammar
//! against `go/testdata/ts-json-model.json`, the committed snapshot of
//! the TypeScript one. Same start, same per-rule node trees, same
//! legend, same ignored set, same `meta.engine`.
//!
//! Compared by VALUE after a JSON round trip, so JSON object key order
//! carries no meaning; and then, because this runtime keeps rule order
//! and the json crate declares it, by rule ORDER too, which Go cannot
//! assert. A byte comparison of the pretty-printed text closes the loop:
//! the struct serializes in the TypeScript key order, so the two texts
//! are the same file.

mod common;

use std::fs;

use tabnas_railroad::GrammarModel;

fn snapshot_text() -> String {
    let path = common::repo_root()
        .join("go")
        .join("testdata")
        .join("ts-json-model.json");
    fs::read_to_string(&path)
        .unwrap_or_else(|error| panic!("read the TypeScript snapshot {}: {error}", path.display()))
}

fn extracted() -> GrammarModel {
    let parser = common::build();
    common::api(&parser).to_json()
}

#[test]
fn matches_the_typescript_model_by_value() {
    let got: serde_json::Value =
        serde_json::from_str(&extracted().to_json()).expect("round-trip the model");
    let want: serde_json::Value =
        serde_json::from_str(&snapshot_text()).expect("parse the TypeScript snapshot");

    // Every field the contract names, including the ones that agree
    // today: a test that only checks what once differed stops covering
    // the rest the moment it agrees.
    for field in ["start", "legend", "ignored"] {
        assert_eq!(
            got[field], want[field],
            "{field} differs from the TypeScript model"
        );
    }

    // meta.engine, not the whole meta map: the contract names the engine
    // and leaves the rest open-ended for a runtime to add to.
    assert!(
        want["meta"]["engine"].is_string(),
        "sanity: the snapshot carries no meta.engine, so comparing it against this port's proves nothing"
    );
    assert_eq!(
        got["meta"]["engine"], want["meta"]["engine"],
        "meta.engine differs from the TypeScript model"
    );

    let got_rules = got["rules"]
        .as_object()
        .expect("this port emitted a rules map");
    let want_rules = want["rules"]
        .as_object()
        .expect("the snapshot holds a rules map");
    assert!(
        !want_rules.is_empty(),
        "sanity: the snapshot has no rules, so comparing against it would pass whatever this port produced"
    );

    let mut got_names: Vec<&String> = got_rules.keys().collect();
    let mut want_names: Vec<&String> = want_rules.keys().collect();
    got_names.sort();
    want_names.sort();
    assert_eq!(got_names, want_names, "rule names differ");
    for name in want_names {
        assert_eq!(
            got_rules[name], want_rules[name],
            "rule {name:?} node tree differs:\n rust: {}\n ts:   {}",
            got_rules[name], want_rules[name]
        );
    }
}

#[test]
fn keeps_the_typescript_rule_order() {
    let model = extracted();
    let got: Vec<&String> = model.rules.keys().collect();
    assert_eq!(got, ["val", "map", "list", "pair", "elem"]);

    // The snapshot's key order is the TypeScript object's insertion
    // order; a decoded model keeps it, so a round trip renders the same.
    let snapshot = GrammarModel::from_json(&snapshot_text()).expect("decode the snapshot");
    let snapshot_order: Vec<&String> = snapshot.rules.keys().collect();
    assert_eq!(got, snapshot_order);
}

#[test]
fn serializes_to_the_typescript_snapshot_text() {
    // `JSON.stringify(model, null, 2)` and serde_json's pretty printer
    // agree on layout, and the struct serializes in the TypeScript key
    // order, so the two files are the same bytes.
    assert_eq!(
        extracted().to_json_pretty().trim_end(),
        snapshot_text().trim_end()
    );
}
