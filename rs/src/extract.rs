/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Build a railroad [`GrammarModel`] by introspecting a live `tabnas`
//! instance. Reads the rule set ([`Tabnas::rule_specs`]) and the resolved
//! options and reverse-maps the alt-based rule machine into railroad
//! constructs (sequence, choice, optional, repetition).
//!
//! The mapping (see the docs, and the tests against `tabnas-json`):
//!
//! - open alt: the first `s.len() - b` token positions are consumed
//!   terminals; a `p` push appends a nonterminal. `b == s.len()` (a pure
//!   peek) consumes nothing, so only the reference is rendered.
//! - several open alts become a choice.
//! - close alt `r: <self>` (plus a guard token) is a repetition
//!   (one-or-more, the guard token on the return path); `r: <other>` is
//!   a continuation. A close alt that consumes a token with no `b` or `r`
//!   is this rule's own closing terminal (appended). A `b` backup close
//!   leaves the token for the parent, so it is dropped. End of source and
//!   a pure pop are dropped.
//! - synthetic helper rules (a name holding `$`, or matching `_gen<digit>`)
//!   are inlined.
//! - normalization factors a common prefix and suffix across choice
//!   branches and turns an empty branch into an optional.
//!
//! This file mirrors `ts/src/extract.ts`. The one structural difference
//! is the introspection shape, the same one the Go port records: this
//! engine resolves an alt's token spec into `Vec<Vec<Tin>>` (the raw
//! `#KEY` / `#VAL` set-name strings are not kept on the live `RuleSpec`),
//! so a readable set name is recovered by matching the resolved tins
//! against the instance's named token sets, disambiguating identical
//! sets (`KEY` against `VAL`) by position role.

use std::collections::{BTreeMap, HashMap, HashSet};

use indexmap::IndexMap;
use tabnas::{AltSpec, MatchTokenMatcher, RuleSpec, Tabnas, Tin, TIN_AA, TIN_CL, TIN_ZZ};

use crate::model::{
    comment, json_string, non_terminal, one_or_more, optional, skip, terminal, GrammarModel,
    LegendEntry, RailroadNode,
};

/// Options for [`extract_grammar`].
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ExtractOptions {
    /// Apply prefix/suffix factoring and empty-branch-to-optional.
    /// Default `true`.
    pub factor: bool,
    /// Override the entry rule (defaults to the instance's
    /// `options.rule.start`).
    pub start: Option<String>,
    /// The named token sets the extractor may recover for a resolved tin
    /// set, in preference order. When `None`, every set the instance
    /// holds except `IGNORE` is a candidate, `VAL` then `KEY` first and
    /// the rest by name. A position immediately followed by a colon
    /// prefers `KEY`.
    pub token_set_names: Option<Vec<String>>,
    /// Human descriptions for named tokens and sets, keyed by bare or
    /// `#`-prefixed name. The Rust analog of the TypeScript
    /// `cfg.tokenDesc` a grammar attaches through `config.modify`, which
    /// this engine has no equivalent of.
    pub token_desc: HashMap<String, String>,
}

impl Default for ExtractOptions {
    fn default() -> Self {
        ExtractOptions {
            factor: true,
            start: None,
            token_set_names: None,
            token_desc: HashMap::new(),
        }
    }
}

impl ExtractOptions {
    /// Extract with the entry rule `start`.
    pub fn with_start(start: impl Into<String>) -> Self {
        ExtractOptions {
            start: Some(start.into()),
            ..Default::default()
        }
    }
}

/// The introspection state threaded through extraction.
struct Ctx<'a> {
    parser: &'a Tabnas,
    rules: IndexMap<&'a str, &'a RuleSpec>,
    factor: bool,
    /// Named token label to meaning.
    legend: BTreeMap<String, String>,
    /// Candidate token-set names, in preference order.
    set_names: Vec<String>,
    /// Every named token set the instance holds.
    token_sets: BTreeMap<String, Vec<Tin>>,
    token_desc: &'a HashMap<String, String>,
}

/// Canonical meanings for the standard tabnas/jsonic token names, used
/// when a grammar references a token whose literal or regex is not
/// recoverable from its configuration. A grammar-supplied description
/// takes precedence, then this table, then engine-derived meanings
/// (regex source, reverse-resolved fixed literal, owning token set).
const CANON: &[(&str, &str)] = &[
    ("OB", "open brace { (start of a map)"),
    ("CB", "close brace } (end of a map)"),
    ("OS", "open square bracket [ (start of a list)"),
    ("CS", "close square bracket ] (end of a list)"),
    ("CL", "colon : (separates a key from its value)"),
    ("CA", "comma , (separates map pairs or list items)"),
    ("CO", "comma , (separates map pairs or list items)"),
    ("SP", "whitespace (spaces or tabs)"),
    ("LN", "newline (line break)"),
    ("CM", "comment"),
    ("NR", "number literal (e.g. 42, -1.5, 1e3)"),
    ("ST", "quoted string (e.g. \"text\")"),
    ("TX", "bare unquoted text (an unquoted word)"),
    ("VL", "value keyword (true, false, null, ...)"),
    ("ZZ", "end of input (no more tokens)"),
    ("AA", "any token (matches anything)"),
    ("BD", "bad input (a character the lexer rejected)"),
    ("UK", "unknown token (unrecognised input)"),
    ("KEY", "map key: bare text, number, string, or keyword"),
    ("VAL", "value: bare text, number, string, or keyword"),
];

fn canon(name: &str) -> Option<&'static str> {
    CANON
        .iter()
        .find(|(canon_name, _)| *canon_name == name)
        .map(|(_, meaning)| *meaning)
}

/// A grammar-supplied description for a token or set name, keyed by
/// `#`-prefixed or bare name.
fn desc_of(name: &str, ctx: &Ctx) -> Option<String> {
    let hashed = format!("#{name}");
    ctx.token_desc
        .get(&hashed)
        .or_else(|| ctx.token_desc.get(name))
        .filter(|d| !d.is_empty())
        .cloned()
}

/// The canonical meaning truncated at its parenthetical, for compact
/// inline use inside a token-set listing.
fn canon_short(name: &str) -> String {
    match canon(name) {
        None => name.to_string(),
        Some(c) => match c.find(" (") {
            Some(at) if at > 0 => c[..at].to_string(),
            _ => c.to_string(),
        },
    }
}

/// Build a railroad [`GrammarModel`] from a live instance.
pub fn extract_grammar(parser: &Tabnas, opts: &ExtractOptions) -> GrammarModel {
    let rules: IndexMap<&str, &RuleSpec> = parser
        .rule_specs()
        .into_iter()
        .map(|spec| (spec.name.as_str(), spec))
        .collect();

    let token_sets: BTreeMap<String, Vec<Tin>> = parser
        .options
        .token_set
        .iter()
        .map(|(name, tins)| (name.clone(), tins.clone()))
        .collect();

    let set_names = match &opts.token_set_names {
        Some(names) => names.clone(),
        None => {
            let mut names: Vec<String> = Vec::new();
            for preferred in ["VAL", "KEY"] {
                if token_sets.contains_key(preferred) {
                    names.push(preferred.to_string());
                }
            }
            let rest: Vec<String> = token_sets
                .keys()
                .filter(|name| *name != "IGNORE" && !names.contains(name))
                .cloned()
                .collect();
            names.extend(rest);
            names
        }
    };

    let mut ctx = Ctx {
        parser,
        rules,
        factor: opts.factor,
        legend: BTreeMap::new(),
        set_names,
        token_sets,
        token_desc: &opts.token_desc,
    };

    // The entry rule; unwrap the synthetic `__start__` end-of-source
    // wrapper when a grammar declares one.
    let mut start = opts
        .start
        .clone()
        .filter(|s| !s.is_empty())
        .or_else(|| Some(parser.options.rule.start.clone()).filter(|s| !s.is_empty()))
        .or_else(|| first_user_rule(&ctx))
        .unwrap_or_default();
    if start == "__start__" {
        if let Some(unwrapped) = ctx
            .rules
            .get("__start__")
            .and_then(|spec| unwrap_start(spec))
        {
            start = unwrapped;
        }
    }

    let names: Vec<String> = ctx
        .rules
        .keys()
        .filter(|name| is_user_rule(name))
        .map(|name| name.to_string())
        .collect();
    let mut rules: IndexMap<String, RailroadNode> = IndexMap::with_capacity(names.len());
    for name in names {
        let node = rule_node(&name, &mut ctx, &HashSet::new());
        rules.insert(name, node);
    }

    // Legend: only the named tokens actually present in the final diagram.
    let mut used: HashSet<String> = HashSet::new();
    for node in rules.values() {
        collect_terminals(node, &mut used);
    }
    let legend: Vec<LegendEntry> = ctx
        .legend
        .iter()
        .filter(|(label, _)| used.contains(*label))
        .map(|(token, meaning)| LegendEntry::new(token, meaning))
        .collect();

    let ignored = build_ignored(&ctx);

    let mut meta = IndexMap::new();
    meta.insert("engine".to_string(), serde_json::Value::from("tabnas"));

    GrammarModel {
        start,
        rules,
        meta,
        legend,
        ignored,
    }
}

/// The IGNORE token set (whitespace, newlines, comments): tokens the
/// lexer silently skips between meaningful tokens. They never appear in
/// a rule, so the diagram cannot show them; they are reported on their
/// own with the same descriptions.
fn build_ignored(ctx: &Ctx) -> Vec<LegendEntry> {
    let Some(ignore) = ctx.token_sets.get("IGNORE") else {
        return Vec::new();
    };
    let mut out: Vec<LegendEntry> = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();
    for &tin in ignore {
        if is_control(tin) {
            continue;
        }
        let name = strip_hash(&tin_name(tin, ctx));
        if name.is_empty() || !seen.insert(name.clone()) {
            continue;
        }
        out.push(LegendEntry::new(name, token_meaning(tin, ctx)));
    }
    out.sort_by(|a, b| a.token.cmp(&b.token));
    out
}

/// Every distinct terminal label used in a node tree.
fn collect_terminals(node: &RailroadNode, out: &mut HashSet<String>) {
    match node {
        RailroadNode::Terminal { text } => {
            out.insert(text.clone());
        }
        RailroadNode::Seq { items }
        | RailroadNode::Choice { items }
        | RailroadNode::Diagram { items } => {
            for item in items {
                collect_terminals(item, out);
            }
        }
        RailroadNode::Optional { item } => collect_terminals(item, out),
        RailroadNode::OneOrMore { item, rep } | RailroadNode::ZeroOrMore { item, rep } => {
            collect_terminals(item, out);
            if let Some(rep) = rep {
                collect_terminals(rep, out);
            }
        }
        _ => {}
    }
}

// ---- rule to node --------------------------------------------------

fn rule_node(name: &str, ctx: &mut Ctx, visited: &HashSet<String>) -> RailroadNode {
    let Some(spec) = ctx.rules.get(name).copied() else {
        return non_terminal(name);
    };
    if visited.contains(name) || visited.len() > 32 {
        return non_terminal(name);
    }
    let mut v2 = visited.clone();
    v2.insert(name.to_string());

    let branches: Vec<RailroadNode> = spec
        .open
        .iter()
        .map(|alt| open_alt_node(alt, ctx, &v2))
        .collect();

    let mut body = match branches.len() {
        0 => skip(),
        1 => branches.into_iter().next().expect("one branch"),
        _ => RailroadNode::Choice { items: branches },
    };

    body = apply_close_alts(body, &spec.close, name, ctx, &v2);
    if ctx.factor {
        body = normalize_node(body);
    }
    body
}

fn open_alt_node(alt: &AltSpec, ctx: &mut Ctx, visited: &HashSet<String>) -> RailroadNode {
    let positions = &alt.s;
    let consumed = positions.len().saturating_sub(numeric_back(alt));

    let mut parts: Vec<RailroadNode> = Vec::new();
    for i in 0..consumed.min(positions.len()) {
        if let Some(node) = position_node(positions, i, ctx) {
            parts.push(node);
        }
    }
    if let Some(reference) = ref_node(
        alt.p.as_deref(),
        alt.p_fn.is_some() || alt.p_match.is_some(),
        ctx,
        visited,
    ) {
        parts.push(reference);
    }

    match parts.len() {
        0 => skip(),
        1 => parts.into_iter().next().expect("one part"),
        _ => RailroadNode::Seq { items: parts },
    }
}

fn apply_close_alts(
    body: RailroadNode,
    close_alts: &[AltSpec],
    rule_name: &str,
    ctx: &mut Ctx,
    visited: &HashSet<String>,
) -> RailroadNode {
    let mut repeat = false;
    let mut rep_sep: Option<RailroadNode> = None;
    let mut closing_term: Option<RailroadNode> = None;
    let mut continuation: Option<RailroadNode> = None;

    for alt in close_alts {
        let positions = &alt.s;
        let back = numeric_back(alt);
        let consumed = positions.len().saturating_sub(back);

        if let Some(target) = alt.r.as_deref() {
            if target == rule_name {
                repeat = true;
                if consumed > 0 {
                    if let Some(node) = position_node(positions, 0, ctx) {
                        rep_sep = Some(node);
                    }
                }
            } else if let Some(node) = ref_node(Some(target), false, ctx, visited) {
                continuation = Some(node);
            }
            continue;
        }

        // No `r`: a backup close leaves the token for the parent, so drop it.
        if back > 0 {
            continue;
        }

        // Consumes token(s) with no backup: this rule's own closing terminal.
        let mut terms: Vec<RailroadNode> = Vec::new();
        for i in 0..consumed.min(positions.len()) {
            if let Some(node) = position_node(positions, i, ctx) {
                terms.push(node);
            }
        }
        match terms.len() {
            0 => {} // A pure pop or end of source: drop.
            1 => closing_term = terms.into_iter().next(),
            _ => closing_term = Some(RailroadNode::Seq { items: terms }),
        }
    }

    let mut result = body;
    if repeat {
        result = one_or_more(result, rep_sep);
    }
    if let Some(continuation) = continuation {
        let mut items = flatten_seq(result);
        items.push(continuation);
        result = RailroadNode::Seq { items };
    }
    if let Some(closing) = closing_term {
        let mut items = flatten_seq(result);
        items.push(closing);
        result = RailroadNode::Seq { items };
    }
    result
}

/// A push or replace reference: inlined when synthetic, otherwise a
/// nonterminal link. A function-valued reference has no name to link,
/// so it renders as the `dynamic` comment, as it does in TypeScript.
fn ref_node(
    target: Option<&str>,
    dynamic: bool,
    ctx: &mut Ctx,
    visited: &HashSet<String>,
) -> Option<RailroadNode> {
    let Some(target) = target else {
        return dynamic.then(|| comment("dynamic"));
    };
    if is_synthetic(target) && ctx.rules.contains_key(target) {
        return Some(rule_node(target, ctx, visited));
    }
    Some(non_terminal(target))
}

// ---- token and position resolution ---------------------------------

/// One token position to a node, or `None` when it holds only control
/// tokens. The surrounding positions disambiguate a key position (one
/// followed by a colon) as `KEY`.
///
/// The TypeScript extractor reads the raw `#KEY` / `#VAL` set name off
/// the alt spec to render a readable set label; this engine resolves
/// that to a tin set, so the set name is recovered by matching the
/// resolved tins to a named token set. The set is preferred over a bare
/// per-tin label whatever the set's size, which is how the json
/// grammar's one-member `KEY` set becomes the `KEY` terminal.
/// Punctuation literals (`{`, which is no set's member) fall through to
/// the per-tin fixed label.
fn position_node(positions: &[Vec<Tin>], i: usize, ctx: &mut Ctx) -> Option<RailroadNode> {
    let useful: Vec<Tin> = positions[i]
        .iter()
        .copied()
        .filter(|tin| !is_control(*tin))
        .collect();
    if useful.is_empty() {
        return None;
    }
    if let Some(set_name) = match_token_set(&useful, positions, i, ctx) {
        let meaning = set_meaning(&set_name, ctx);
        ctx.legend.insert(set_name.clone(), meaning);
        return Some(terminal(set_name));
    }
    if useful.len() == 1 {
        return Some(terminal(named_label(useful[0], ctx)));
    }
    let items: Vec<RailroadNode> = useful
        .iter()
        .map(|tin| terminal(named_label(*tin, ctx)))
        .collect();
    Some(RailroadNode::Choice { items })
}

/// The token label, additionally recording a legend entry when the label
/// is a named token rather than a self-explanatory punctuation literal.
fn named_label(tin: Tin, ctx: &mut Ctx) -> String {
    let label = token_label(tin, ctx);
    if ctx.parser.fixed_source(tin) != Some(label.as_str()) {
        let meaning = token_meaning(tin, ctx);
        ctx.legend.insert(label.clone(), meaning);
    }
    label
}

/// The human meaning of a named token: a grammar-supplied description,
/// else the canonical standard name, else a regex match, else a
/// reverse-resolved fixed literal, else the token set it belongs to,
/// else a bare label.
fn token_meaning(tin: Tin, ctx: &Ctx) -> String {
    let name = strip_hash(&tin_name(tin, ctx));
    if let Some(desc) = desc_of(&name, ctx) {
        return desc;
    }
    if let Some(meaning) = canon(&name) {
        return meaning.to_string();
    }
    if let Some(source) = regex_source(tin, ctx) {
        return format!("text matching /{source}/");
    }
    if let Some(source) = ctx.parser.fixed_source(tin) {
        return format!("literal {}", json_string(source));
    }
    if let Some(owner) = sole_set_of(tin, ctx) {
        return format!("part of {owner}");
    }
    if name.is_empty() {
        "token".to_string()
    } else {
        format!("{name} token")
    }
}

/// The human meaning of a named token set: a grammar-supplied
/// description, else canonical, else a deduplicated list of its members'
/// short meanings.
fn set_meaning(set_name: &str, ctx: &Ctx) -> String {
    if let Some(desc) = desc_of(set_name, ctx) {
        return desc;
    }
    if let Some(meaning) = canon(set_name) {
        return meaning.to_string();
    }
    let mut seen: HashSet<String> = HashSet::new();
    let mut members: Vec<String> = Vec::new();
    for &tin in ctx
        .token_sets
        .get(set_name)
        .map(Vec::as_slice)
        .unwrap_or(&[])
    {
        let n = strip_hash(&tin_name(tin, ctx));
        let label = desc_of(&n, ctx).unwrap_or_else(|| canon_short(&n));
        if !label.is_empty() && seen.insert(label.clone()) {
            members.push(label);
        }
    }
    if members.is_empty() {
        "token set".to_string()
    } else {
        format!("one of: {}", members.join(", "))
    }
}

/// The single named token set a tin belongs to (ignoring `IGNORE`), or
/// `None` when it is in none or in several: a last-resort meaning for an
/// otherwise bare token.
fn sole_set_of(tin: Tin, ctx: &Ctx) -> Option<String> {
    let mut found: Option<String> = None;
    for (name, members) in &ctx.token_sets {
        if name == "IGNORE" || !members.contains(&tin) {
            continue;
        }
        if found.is_some() {
            return None; // In more than one set: ambiguous.
        }
        found = Some(name.clone());
    }
    found
}

fn token_label(tin: Tin, ctx: &Ctx) -> String {
    if let Some(source) = ctx.parser.fixed_source(tin) {
        return source.to_string();
    }
    let name = tin_name(tin, ctx);
    if !name.is_empty() {
        return strip_hash(&name);
    }
    if let Some(source) = regex_source(tin, ctx) {
        return source;
    }
    format!("#{tin}")
}

/// The engine's name for a tin, or empty when it has none. The engine
/// answers `#UNKNOWN` for a tin it does not know, which is not a name.
fn tin_name(tin: Tin, ctx: &Ctx) -> String {
    let name = ctx.parser.token_name(tin);
    if name == "#UNKNOWN" {
        String::new()
    } else {
        name
    }
}

/// The source of the regex a match token is lexed by, trimmed of its
/// anchors, when the tin is such a token.
fn regex_source(tin: Tin, ctx: &Ctx) -> Option<String> {
    ctx.parser
        .options
        .match_tokens
        .values()
        .find(|token| token.tin == tin)
        .and_then(|token| match &token.matcher {
            MatchTokenMatcher::Regex(regex) => Some(pretty_source(regex.as_str())),
            _ => None,
        })
}

/// A named token set whose members equal the given tins. When several
/// match (`KEY` and `VAL` share members on a bare engine), `KEY` is
/// preferred for a position immediately followed by a colon (a map-key
/// position), else the first candidate in preference order.
fn match_token_set(tins: &[Tin], positions: &[Vec<Tin>], i: usize, ctx: &Ctx) -> Option<String> {
    let want: HashSet<Tin> = tins.iter().copied().collect();
    let matched: Vec<&String> = ctx
        .set_names
        .iter()
        .filter(|name| {
            ctx.token_sets.get(*name).is_some_and(|members| {
                members.len() == want.len() && members.iter().all(|m| want.contains(m))
            })
        })
        .collect();
    match matched.len() {
        0 => None,
        1 => Some(matched[0].clone()),
        _ => {
            if is_key_position(positions, i) {
                if let Some(key) = matched.iter().find(|name| *name == &"KEY") {
                    return Some((*key).clone());
                }
            }
            Some(matched[0].clone())
        }
    }
}

/// Whether the slot at `i` is immediately followed by a colon slot, the
/// shape of a map key (`KEY ":" ...`).
fn is_key_position(positions: &[Vec<Tin>], i: usize) -> bool {
    positions
        .get(i + 1)
        .is_some_and(|next| next.contains(&TIN_CL))
}

// ---- normalization passes ------------------------------------------

fn normalize_node(node: RailroadNode) -> RailroadNode {
    match node {
        RailroadNode::Seq { items } => seq_of(items.into_iter().map(normalize_node).collect()),
        RailroadNode::Choice { items } => {
            factor_choice(items.into_iter().map(normalize_node).collect())
        }
        RailroadNode::Optional { item } => optional(normalize_node(*item)),
        RailroadNode::OneOrMore { item, rep } => {
            one_or_more(normalize_node(*item), rep.map(|r| normalize_node(*r)))
        }
        RailroadNode::ZeroOrMore { item, rep } => RailroadNode::ZeroOrMore {
            item: Box::new(normalize_node(*item)),
            rep: rep.map(|r| Box::new(normalize_node(*r))),
        },
        RailroadNode::Diagram { items } => RailroadNode::Diagram {
            items: items.into_iter().map(normalize_node).collect(),
        },
        other => other,
    }
}

fn factor_choice(raw_branches: Vec<RailroadNode>) -> RailroadNode {
    // Deduplicate identical branches.
    let mut branches: Vec<RailroadNode> = Vec::new();
    for b in raw_branches {
        if !branches.contains(&b) {
            branches.push(b);
        }
    }
    if branches.len() == 1 {
        return branches.remove(0);
    }

    // Separate empty (skip) branches.
    let mut has_empty = false;
    let non_empty: Vec<RailroadNode> = branches
        .into_iter()
        .filter(|b| {
            if matches!(b, RailroadNode::Skip) {
                has_empty = true;
                false
            } else {
                true
            }
        })
        .collect();
    if non_empty.is_empty() {
        return skip();
    }

    let seqs: Vec<Vec<RailroadNode>> = non_empty.into_iter().map(as_seq_items).collect();

    // Common prefix.
    let mut prefix: Vec<RailroadNode> = Vec::new();
    let mut i = 0;
    while let Some(first) = seqs[0].get(i) {
        if !seqs.iter().all(|s| s.get(i) == Some(first)) {
            break;
        }
        prefix.push(first.clone());
        i += 1;
    }

    // Common suffix, not overlapping the prefix.
    let mut suffix: Vec<RailroadNode> = Vec::new();
    let min_tail = seqs
        .iter()
        .map(|s| s.len() - prefix.len())
        .min()
        .unwrap_or(0);
    for k in 1..=min_tail {
        let first = &seqs[0][seqs[0].len() - k];
        if !seqs.iter().all(|s| &s[s.len() - k] == first) {
            break;
        }
        suffix.insert(0, first.clone());
    }

    // Remainders between prefix and suffix.
    let mut rem_empty = false;
    let mut rem_nodes: Vec<RailroadNode> = Vec::new();
    for s in &seqs {
        let rem = seq_of(s[prefix.len()..s.len() - suffix.len()].to_vec());
        if matches!(rem, RailroadNode::Skip) {
            rem_empty = true;
            continue;
        }
        if !rem_nodes.contains(&rem) {
            rem_nodes.push(rem);
        }
    }

    let mut core = match rem_nodes.len() {
        0 => skip(),
        1 => rem_nodes.remove(0),
        _ => RailroadNode::Choice { items: rem_nodes },
    };

    if (has_empty || rem_empty) && !matches!(core, RailroadNode::Skip) {
        core = optional(core);
    }

    let mut items = prefix;
    if !matches!(core, RailroadNode::Skip) {
        items.push(core);
    }
    items.extend(suffix);
    seq_of(items)
}

fn as_seq_items(node: RailroadNode) -> Vec<RailroadNode> {
    match node {
        RailroadNode::Seq { items } => items,
        RailroadNode::Skip => Vec::new(),
        other => vec![other],
    }
}

fn flatten_seq(node: RailroadNode) -> Vec<RailroadNode> {
    as_seq_items(node)
}

fn seq_of(items: Vec<RailroadNode>) -> RailroadNode {
    let mut flat: Vec<RailroadNode> = Vec::new();
    for item in items {
        match item {
            RailroadNode::Seq { items } => flat.extend(items),
            RailroadNode::Skip => {}
            other => flat.push(other),
        }
    }
    match flat.len() {
        0 => skip(),
        1 => flat.remove(0),
        _ => RailroadNode::Seq { items: flat },
    }
}

// ---- small helpers -------------------------------------------------

fn is_control(tin: Tin) -> bool {
    tin == TIN_ZZ || tin == TIN_AA
}

/// The alt's backtrack count. A function-valued backtrack is taken as a
/// single-token peek, as the TypeScript `numericBack` takes one.
fn numeric_back(alt: &AltSpec) -> usize {
    if alt.b_fn.is_some() || alt.b_match.is_some() {
        1
    } else {
        alt.b
    }
}

/// A synthetic helper rule: its name holds `$`, or starts `_gen<digit>`.
fn is_synthetic(name: &str) -> bool {
    name.contains('$')
        || name
            .strip_prefix("_gen")
            .and_then(|rest| rest.chars().next())
            .is_some_and(|c| c.is_ascii_digit())
}

fn is_user_rule(name: &str) -> bool {
    name != "__start__" && !is_synthetic(name)
}

/// The fallback entry rule when neither the caller nor the options name
/// one: the FIRST user rule the grammar declared, the one the TypeScript
/// `Object.keys(rsm).find(isUserRule)` picks.
fn first_user_rule(ctx: &Ctx) -> Option<String> {
    ctx.rules
        .keys()
        .find(|name| is_user_rule(name))
        .map(|name| name.to_string())
}

fn unwrap_start(spec: &RuleSpec) -> Option<String> {
    spec.open.first().and_then(|alt| alt.p.clone())
}

fn strip_hash(s: &str) -> String {
    s.strip_prefix('#').unwrap_or(s).to_string()
}

fn pretty_source(source: &str) -> String {
    let source = source.strip_prefix('^').unwrap_or(source);
    source.strip_suffix('$').unwrap_or(source).to_string()
}
