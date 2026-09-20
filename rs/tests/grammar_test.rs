/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The port of `go/grammar_test.go` (itself the port of
//! `ts/test/grammar.test.js`): grammar-driven extraction and rendering,
//! validated against the Rust `tabnas-json` plugin, plus the example
//! outputs the READMEs show, which TypeScript generated from the same
//! grammar.

mod common;

use std::collections::HashMap;
use std::fs;

use tabnas_railroad::{
    extract_grammar, model_to_ascii, model_to_svg, non_terminal, terminal, AsciiOptions,
    ExtractOptions, GrammarModel, RailroadNode,
};

fn model() -> GrammarModel {
    let parser = common::build();
    common::api(&parser).to_json()
}

#[test]
fn extracts_the_rule_set_and_start() {
    let model = model();
    assert_eq!(model.start, "val");
    let mut names: Vec<&String> = model.rules.keys().collect();
    names.sort();
    assert_eq!(names, ["elem", "list", "map", "pair", "val"]);
    assert_eq!(model.engine(), Some("tabnas"));
}

#[test]
fn val_is_a_choice_of_map_list_val() {
    let model = model();
    let RailroadNode::Choice { items } = &model.rules["val"] else {
        panic!("val.kind = {}, want choice", model.rules["val"].kind());
    };
    assert!(items.contains(&non_terminal("map")));
    assert!(items.contains(&non_terminal("list")));
    assert!(items.contains(&terminal("VAL")));
}

fn container_shape(rule: &str, open: &str, inner: &str, close: &str) {
    let model = model();
    let RailroadNode::Seq { items } = &model.rules[rule] else {
        panic!("{rule}.kind = {}, want seq", model.rules[rule].kind());
    };
    assert_eq!(items[0], terminal(open));
    assert_eq!(items[1].kind(), "optional");
    assert_eq!(items[1].item(), Some(&non_terminal(inner)));
    assert_eq!(items[2], terminal(close));
}

#[test]
fn map_is_brace_optional_pair_brace() {
    container_shape("map", "{", "pair", "}");
}

#[test]
fn list_is_bracket_optional_elem_bracket() {
    container_shape("list", "[", "elem", "]");
}

#[test]
fn pair_is_key_colon_val_repeated_with_comma() {
    let model = model();
    let RailroadNode::OneOrMore { item, rep } = &model.rules["pair"] else {
        panic!("pair.kind = {}, want oneOrMore", model.rules["pair"].kind());
    };
    assert_eq!(rep.as_deref(), Some(&terminal(",")));
    let RailroadNode::Seq { items } = item.as_ref() else {
        panic!("pair.item.kind = {}, want seq", item.kind());
    };
    assert_eq!(items[0], terminal("KEY"));
    assert_eq!(items[1], terminal(":"));
    assert_eq!(items[2], non_terminal("val"));
}

#[test]
fn elem_is_val_repeated_with_comma() {
    let model = model();
    let RailroadNode::OneOrMore { item, rep } = &model.rules["elem"] else {
        panic!("elem.kind = {}, want oneOrMore", model.rules["elem"].kind());
    };
    assert_eq!(item.as_ref(), &non_terminal("val"));
    assert_eq!(rep.as_deref(), Some(&terminal(",")));
}

#[test]
fn bare_extract_grammar_matches_the_api() {
    let parser = common::build();
    let bare = extract_grammar(&parser, &ExtractOptions::default());
    let api = common::api(&parser).to_json();
    assert_eq!(bare, api);
    assert_eq!(bare.to_json(), api.to_json());
}

#[test]
fn extraction_needs_no_plugin() {
    // Decoration is lazy in TypeScript; here the API binds to any
    // instance, so a bare json parser extracts the same model.
    let parser = tabnas_json::make();
    assert!(!tabnas_railroad::installed(&parser));
    assert_eq!(tabnas_railroad::of(&parser).to_json(), model());
}

#[test]
fn factoring_can_be_switched_off() {
    let parser = common::build();
    let raw = extract_grammar(
        &parser,
        &ExtractOptions {
            factor: false,
            ..Default::default()
        },
    );
    // Unfactored, map is the raw choice of its two open alts followed by
    // the closing brace: `("{" | "{" pair) "}"`.
    assert_eq!(
        tabnas_railroad::to_text(&raw.rules["map"]),
        r#"("{" | "{" pair) "}""#
    );
    assert_eq!(
        tabnas_railroad::to_text(&model().rules["map"]),
        r#""{" [pair] "}""#
    );
}

#[test]
fn token_desc_overrides_the_legend() {
    let parser = common::build();
    let mut token_desc = HashMap::new();
    token_desc.insert("#KEY".to_string(), "a key".to_string());
    token_desc.insert("SP".to_string(), "a space".to_string());
    let model = extract_grammar(
        &parser,
        &ExtractOptions {
            token_desc,
            ..Default::default()
        },
    );
    let legend: HashMap<&str, &str> = model
        .legend
        .iter()
        .map(|e| (e.token.as_str(), e.meaning.as_str()))
        .collect();
    assert_eq!(legend["KEY"], "a key");
    assert!(legend["VAL"].contains("value"));
    let ignored: HashMap<&str, &str> = model
        .ignored
        .iter()
        .map(|e| (e.token.as_str(), e.meaning.as_str()))
        .collect();
    assert_eq!(ignored["SP"], "a space");
}

// ---- whole-grammar rendering ----------------------------------------

#[test]
fn svg_is_well_formed_anchored_and_linked() {
    let parser = common::build();
    let svg = common::api(&parser).to_svg();
    assert!(svg.starts_with("<svg "));
    assert!(svg.ends_with("</svg>"));
    let w = common::svg_attr(&svg, "width");
    let h = common::svg_attr(&svg, "height");
    assert!(w > 0 && h > 0, "width/height must be positive, got {w}/{h}");
    assert!(h > w, "expected a taller-than-wide diagram, got {w}x{h}");
    for rule in ["val", "map", "list", "pair", "elem"] {
        assert!(
            svg.contains(&format!(r#"id="{rule}""#)),
            "missing track anchor id={rule:?}"
        );
    }
    assert!(
        svg.contains(r##"<a href="#map""##),
        "nonterminal map should link"
    );
    assert!(
        svg.contains(r##"<a href="#val""##),
        "nonterminal val should link"
    );
}

#[test]
fn ascii_contains_every_rule_name() {
    let parser = common::build();
    let ascii = common::api(&parser).to_ascii(&AsciiOptions::default());
    for rule in ["val", "map", "list", "pair", "elem"] {
        assert!(
            ascii.contains(&format!("{rule}:")),
            "missing rule heading {rule}:"
        );
    }
}

#[test]
fn ascii_plain_is_pure_ascii() {
    let parser = common::build();
    let ascii = common::api(&parser).to_ascii(&AsciiOptions::plain());
    assert!(ascii.is_ascii(), "expected pure ASCII output");
}

#[test]
fn model_round_trips_to_the_same_output() {
    let model = model();
    let clone = GrammarModel::from_json(&model.to_json()).expect("the model decodes");
    assert_eq!(clone, model);
    assert_eq!(model_to_svg(&clone), model_to_svg(&model));
    assert_eq!(
        model_to_ascii(&clone, &AsciiOptions::default()),
        model_to_ascii(&model, &AsciiOptions::default())
    );
    let pretty = GrammarModel::from_json(&model.to_json_pretty()).expect("the model decodes");
    assert_eq!(pretty, model);
}

#[test]
fn emits_a_token_legend() {
    let model = model();
    assert!(
        !model.legend.is_empty(),
        "model should carry a token legend"
    );
    let meaning: HashMap<&str, &str> = model
        .legend
        .iter()
        .map(|e| (e.token.as_str(), e.meaning.as_str()))
        .collect();
    // json renders { / : / } as literals, but the KEY/VAL token sets
    // show as names and therefore need a key entry.
    assert!(meaning.contains_key("KEY"), "legend should explain KEY");
    assert!(meaning.contains_key("VAL"), "legend should explain VAL");
    assert!(meaning["VAL"].contains("value"));
    assert!(model_to_ascii(&model, &AsciiOptions::default()).contains("Tokens:"));
    assert!(model_to_svg(&model).contains(">Tokens<"));
}

#[test]
fn reports_ignored_tokens() {
    let model = model();
    assert!(
        !model.ignored.is_empty(),
        "model should carry the ignored-token set"
    );
    let tokens: Vec<&str> = model.ignored.iter().map(|e| e.token.as_str()).collect();
    assert!(tokens.contains(&"SP"), "SP should be reported as ignored");
    assert!(tokens.contains(&"LN"), "LN should be reported as ignored");
    assert!(
        model.ignored.iter().all(|e| !e.meaning.is_empty()),
        "every ignored token should carry a meaning"
    );
    assert!(model_to_ascii(&model, &AsciiOptions::default()).contains("Ignored tokens:"));
    assert!(model_to_svg(&model).contains(">Ignored tokens<"));
}

// ---- the committed examples -----------------------------------------

fn example(name: &str) -> String {
    let path = common::repo_root().join("examples").join(name);
    fs::read_to_string(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()))
}

/// `examples/json-grammar.txt` is the TypeScript rendering of the json
/// grammar. Same model, same renderer, same bytes.
#[test]
fn ascii_matches_the_typescript_example() {
    let ascii = model_to_ascii(&model(), &AsciiOptions::default());
    assert_eq!(ascii.trim_end(), example("json-grammar.txt").trim_end());
}

/// `examples/json-grammar.svg` likewise: the two runtimes produce
/// byte-identical SVG for the same model.
#[test]
fn svg_matches_the_typescript_example() {
    let svg = model_to_svg(&model());
    assert_eq!(svg.trim_end(), example("json-grammar.svg").trim_end());
}
