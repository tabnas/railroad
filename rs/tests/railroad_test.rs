/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The port of `go/railroad_test.go` (itself the port of
//! `ts/test/railroad.test.js`): node-level model construction, the text
//! emitter, the single-node SVG and ASCII renderers, the plugin mark,
//! rule ordering, and the error cases.

mod common;

use regex::Regex;
use tabnas::{GrammarSpec, Tabnas};
use tabnas_railroad::{
    choice, diagram, extract_grammar, model_to_ascii, model_to_svg, node_equal, non_terminal,
    one_or_more, optional, render_node_ascii, render_node_svg, sequence, skip, terminal, to_text,
    zero_or_more, AsciiOptions, ExtractOptions, GrammarModel, RailroadDecoration, RailroadError,
    RailroadNode, SvgOptions, DECORATION_NAME,
};

// ---- plugin load ----------------------------------------------------

#[test]
fn plugin_marks_the_instance() {
    let mut parser = Tabnas::new();
    // Before loading, the mark is absent, and the API still binds.
    assert!(!tabnas_railroad::installed(&parser));
    assert_eq!(
        tabnas_railroad::of(&parser).render_node_text(&terminal("x")),
        "\"x\""
    );

    tabnas_railroad::railroad(&mut parser).expect("the plugin installs");
    assert!(tabnas_railroad::installed(&parser));
    assert_eq!(
        parser.decoration::<RailroadDecoration>(DECORATION_NAME),
        Some(&RailroadDecoration)
    );
}

#[test]
fn loads_through_use_plugin() {
    let mut parser = Tabnas::new();
    parser
        .use_plugin(tabnas_railroad::plugin(), None)
        .expect("the plugin installs");
    assert!(tabnas_railroad::installed(&parser));
}

#[test]
fn derived_instances_inherit_the_mark() {
    let mut parser = Tabnas::new();
    tabnas_railroad::railroad(&mut parser).expect("the plugin installs");
    let child = parser.derive(|_| {}).expect("the child derives");
    assert!(tabnas_railroad::installed(&child));
}

// ---- text emitter ---------------------------------------------------

#[test]
fn text_emitter() {
    let cases: Vec<(RailroadNode, &str)> = vec![
        (terminal("hi"), "\"hi\""),
        (non_terminal("expr"), "expr"),
        (sequence(["a", "b"]), "\"a\" \"b\""),
        (choice(["a", "b"]).unwrap(), "(\"a\" | \"b\")"),
        (optional("a"), "[\"a\"]"),
        (one_or_more("a", None), "\"a\"+"),
        (zero_or_more("a", None), "{\"a\"}"),
        (one_or_more("a", Some(terminal(","))), "\"a\"+ /* \",\" */"),
        (tabnas_railroad::comment("note"), "/* note */"),
        (skip(), ""),
    ];
    for (node, want) in cases {
        assert_eq!(to_text(&node), want, "to_text({node:?})");
    }
}

// ---- svg node renderer ----------------------------------------------

#[test]
fn svg_node_is_well_formed() {
    let svg = render_node_svg(
        &diagram([sequence([terminal("GET"), non_terminal("path")])]),
        &SvgOptions::default(),
    );
    assert!(svg.starts_with("<svg "), "svg not well-formed: {svg:.40}");
    assert!(svg.ends_with("</svg>"), "svg not well-formed");
    assert!(common::svg_attr(&svg, "width") > 0);
    assert!(common::svg_attr(&svg, "height") > 0);
    assert!(svg.contains("GET") && svg.contains("path") && svg.contains("<rect"));
}

#[test]
fn svg_nested_renders() {
    let node = diagram([sequence([
        terminal("["),
        optional(sequence([
            non_terminal("item"),
            zero_or_more(sequence([terminal(","), non_terminal("item")]), None),
        ])),
        terminal("]"),
    ])]);
    assert!(render_node_svg(&node, &SvgOptions::default()).starts_with("<svg "));
}

#[test]
fn svg_stacks_a_sequence_without_overlap() {
    let svg = render_node_svg(&sequence(["a", "b", "c"]), &SvgOptions::default());
    let re = Regex::new(r#"<rect[^>]*\sy="([\d.]+)"[^>]*\sheight="([\d.]+)""#).unwrap();
    let mut bands: Vec<(f64, f64)> = re
        .captures_iter(&svg)
        .map(|m| {
            let y: f64 = m[1].parse().unwrap();
            let h: f64 = m[2].parse().unwrap();
            (y, y + h)
        })
        .collect();
    assert_eq!(bands.len(), 3, "expected 3 rects");
    bands.sort_by(|a, b| a.0.partial_cmp(&b.0).unwrap());
    for pair in bands.windows(2) {
        assert!(
            pair[1].0 >= pair[0].1,
            "sequence boxes overlap vertically: {bands:?}"
        );
    }
}

#[test]
fn svg_links_nonterminals_through_link_for() {
    let opts = SvgOptions::with_link_for(|name| Some(format!("#rule-{name}")));
    let svg = render_node_svg(&non_terminal("expr"), &opts);
    assert!(svg.contains(r##"<a href="#rule-expr">"##));
    let plain = render_node_svg(&non_terminal("expr"), &SvgOptions::default());
    assert!(!plain.contains("<a "));
}

#[test]
fn svg_escapes_markup_in_labels() {
    let svg = render_node_svg(&terminal("<&>"), &SvgOptions::default());
    assert!(svg.contains("&lt;&amp;&gt;"));
    assert!(!svg.contains("<&>"));
}

// ---- ascii node renderer --------------------------------------------

#[test]
fn ascii_node_sequence() {
    let out = render_node_ascii(
        &diagram([sequence([terminal("a"), non_terminal("b")])]),
        &AsciiOptions::default(),
    );
    assert!(out.contains("\"a\"") && out.contains('b'), "{out}");
}

#[test]
fn ascii_plain_is_pure_ascii() {
    let out = render_node_ascii(&choice(["a", "b"]).unwrap(), &AsciiOptions::plain());
    assert!(out.is_ascii(), "expected pure ASCII, got {out}");
}

// ---- errors ---------------------------------------------------------

#[test]
fn choice_with_no_branches_errors() {
    let error = choice(Vec::<RailroadNode>::new()).unwrap_err();
    assert_eq!(
        error,
        RailroadError::new("railroad: choice needs at least one branch")
    );
    assert_eq!(
        error.to_string(),
        "railroad: choice needs at least one branch"
    );
}

#[test]
fn unknown_node_kind_is_rejected_on_decode() {
    // TypeScript rejects `{ kind: 'bogus' }` when it reaches a renderer;
    // a typed node cannot hold an unknown kind, so this port rejects it
    // when the JSON is decoded, before it reaches one.
    let error = RailroadNode::from_json(r#"{"kind":"bogus"}"#).unwrap_err();
    assert!(error.message.starts_with("railroad: invalid diagram node"));
    assert_eq!(error.node, Some(serde_json::json!({"kind":"bogus"})));
}

#[test]
fn invalid_node_values_are_rejected_on_decode() {
    // The analog of Sequence(null) and Norm(42): not a node at all.
    for bad in ["null", "42", "{}", "{\"text\":\"x\"}", "[]"] {
        let error = RailroadNode::from_json(bad).unwrap_err();
        assert!(
            error.message.starts_with("railroad: invalid diagram node"),
            "{bad}: {error}"
        );
    }
    assert!(GrammarModel::from_json("[]").is_err());
}

// ---- whole-model rule ordering --------------------------------------

/// The renderers must emit rules in the model's own order, hoisting
/// `start` to the front, never alphabetically. This pins the renderer
/// half with a hand-built model whose order is deliberately neither
/// alphabetical nor start-first.
#[test]
fn renderers_honour_declared_rule_order() {
    let mut model = GrammarModel {
        start: "mid".to_string(),
        ..Default::default()
    };
    model.rules.insert("zebra".to_string(), terminal("z"));
    model.rules.insert("alpha".to_string(), terminal("a"));
    model.rules.insert("mid".to_string(), terminal("m"));
    let want = ["mid", "zebra", "alpha"];

    assert_eq!(model.rule_order(), want);

    let ascii = model_to_ascii(&model, &AsciiOptions::default());
    let got: Vec<&str> = ascii
        .lines()
        .filter(|line| line.ends_with(':') && !line.contains([' ', '\t']))
        .map(|line| line.trim_end_matches(':'))
        .collect();
    assert_eq!(got, want, "ascii rule order");

    let svg = model_to_svg(&model);
    let re = Regex::new(r#"<g id="([^"]+)">"#).unwrap();
    let got: Vec<&str> = re
        .captures_iter(&svg)
        .map(|m| m.get(1).unwrap().as_str())
        .collect();
    assert_eq!(got, want, "svg rule order");
}

/// Extraction must put rules in the model in the order the GRAMMAR
/// declared them, matching the TypeScript `Object.keys(rsm)` walk. The
/// engine keeps insertion order; a serialized grammar states it in
/// `ruleOrder`. The names are chosen so declaration order is neither
/// alphabetical nor its reverse, so a sort in either direction fails.
#[test]
fn extraction_honours_declaration_order() {
    let declared = ["zebra", "alpha", "mid"];
    let mut parser = Tabnas::new();
    let spec = GrammarSpec::from_value(serde_json::json!({
        "v": 2,
        "rule": {
            "zebra": { "open": [ { "s": "#TX", "p": "alpha" } ] },
            "alpha": { "open": [ { "s": "#TX", "p": "mid" } ] },
            "mid":   { "open": [ { "s": "#TX" } ] },
        },
        "ruleOrder": declared,
    }))
    .expect("the grammar document is valid");
    parser.grammar(&spec).expect("the grammar installs");

    let model = extract_grammar(&parser, &ExtractOptions::with_start("mid"));
    let got: Vec<&String> = model.rules.keys().collect();
    assert_eq!(got, declared);
    assert_eq!(model.start, "mid");
    assert_eq!(model.rule_order(), ["mid", "zebra", "alpha"]);

    // The last-resort entry rule (neither the caller nor the options
    // name one) is the FIRST rule declared, not the alphabetically first.
    parser.options.rule.start = String::new();
    let model = extract_grammar(&parser, &ExtractOptions::default());
    assert_eq!(model.start, "zebra");

    let ascii = model_to_ascii(&model, &AsciiOptions::default());
    let got: Vec<&str> = ascii
        .lines()
        .filter(|line| line.ends_with(':') && !line.contains([' ', '\t']))
        .map(|line| line.trim_end_matches(':'))
        .filter(|name| model.rules.contains_key(*name))
        .collect();
    assert_eq!(got, ["zebra", "alpha", "mid"]);
}

// ---- norm and node_equal --------------------------------------------

#[test]
fn a_bare_string_is_a_terminal() {
    // The TypeScript `norm` coercion of its Item union.
    let node: RailroadNode = "(".into();
    assert!(node_equal(&node, &terminal("(")));
    let node: RailroadNode = String::from("expr").into();
    assert_eq!(node, terminal("expr"));
    assert_eq!(
        sequence(["(", "expr", ")"]),
        sequence([terminal("("), terminal("expr"), terminal(")")])
    );
}

#[test]
fn node_equal_is_structural() {
    let a = choice([
        terminal("a"),
        optional(non_terminal("b")),
        one_or_more(terminal("c"), Some(terminal(","))),
    ])
    .unwrap();
    let b = choice([
        terminal("a"),
        optional(non_terminal("b")),
        one_or_more(terminal("c"), Some(terminal(","))),
    ])
    .unwrap();
    assert!(node_equal(&a, &b));
    assert!(node_equal(&skip(), &skip()));
    assert!(!node_equal(&terminal("x"), &non_terminal("x")));
    assert!(!node_equal(&terminal("x"), &terminal("y")));
    assert!(!node_equal(&a, &choice([terminal("a")]).unwrap()));
    assert!(!node_equal(
        &one_or_more(terminal("c"), Some(terminal(","))),
        &one_or_more(terminal("c"), None)
    ));
}

#[test]
fn nodes_round_trip_through_json() {
    let node = diagram([sequence([
        terminal("["),
        optional(sequence([
            non_terminal("item"),
            zero_or_more(sequence([terminal(","), non_terminal("item")]), None),
        ])),
        terminal("]"),
        one_or_more(terminal("x"), Some(tabnas_railroad::comment("sep"))),
        skip(),
    ])]);
    let json = node.to_json();
    assert_eq!(RailroadNode::from_json(&json).unwrap(), node);
    // The shapes are the TypeScript object shapes exactly.
    assert_eq!(terminal("x").to_json(), r#"{"kind":"terminal","text":"x"}"#);
    assert_eq!(skip().to_json(), r#"{"kind":"skip"}"#);
    assert_eq!(
        one_or_more("x", None).to_json(),
        r#"{"kind":"oneOrMore","item":{"kind":"terminal","text":"x"}}"#
    );
    assert_eq!(
        RailroadNode::from_json(r#"{"kind":"oneOrMore","item":{"kind":"skip"},"rep":null}"#)
            .unwrap(),
        one_or_more(skip(), None)
    );
}

#[test]
fn accessors_read_each_shape() {
    assert_eq!(terminal("x").kind(), "terminal");
    assert_eq!(non_terminal("x").kind(), "nonterminal");
    assert_eq!(one_or_more("x", None).kind(), "oneOrMore");
    assert_eq!(zero_or_more("x", None).kind(), "zeroOrMore");
    assert_eq!(terminal("x").text(), Some("x"));
    assert_eq!(skip().text(), None);
    assert_eq!(sequence(["a"]).items().map(<[RailroadNode]>::len), Some(1));
    assert_eq!(optional("a").item(), Some(&terminal("a")));
    assert_eq!(
        one_or_more("a", Some(terminal(","))).rep(),
        Some(&terminal(","))
    );
    assert_eq!(one_or_more("a", None).rep(), None);
}

#[test]
fn version_is_exported() {
    assert_eq!(tabnas_railroad::VERSION, "0.3.6");
}
