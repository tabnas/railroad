/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! The railroad diagram model: a small, engine-agnostic tree of nodes
//! (terminal, nonterminal, sequence, choice, optional, repetition) plus
//! the [`GrammarModel`] envelope, one node per grammar rule. The model is
//! pure, JSON-serializable data: the interchange format the SVG and
//! ASCII renderers consume and that [`crate::extract_grammar`] produces
//! from a live tabnas instance.
//!
//! This file mirrors `ts/src/model.ts`.

use std::fmt;

use indexmap::IndexMap;
use serde::{Deserialize, Serialize};

/// One node of a railroad diagram tree.
///
/// The TypeScript model is a discriminated union on `kind`; this enum
/// serializes to exactly those object shapes (a terminal is
/// `{"kind":"terminal","text":"x"}`, a skip is `{"kind":"skip"}`, and
/// `rep` is omitted when absent), so a node decoded from any runtime is
/// the same node here.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "kind")]
pub enum RailroadNode {
    /// A literal or named token.
    #[serde(rename = "terminal")]
    Terminal {
        /// The token label.
        text: String,
    },
    /// A rule reference.
    #[serde(rename = "nonterminal")]
    NonTerminal {
        /// The rule name.
        text: String,
    },
    /// An inline comment.
    #[serde(rename = "comment")]
    Comment {
        /// The comment text.
        text: String,
    },
    /// A bypass: an empty path.
    #[serde(rename = "skip")]
    Skip,
    /// Items in order.
    #[serde(rename = "seq")]
    Seq {
        /// The items, top to bottom.
        items: Vec<RailroadNode>,
    },
    /// One of several branches.
    #[serde(rename = "choice")]
    Choice {
        /// The branches, left to right.
        items: Vec<RailroadNode>,
    },
    /// An item that may be bypassed.
    #[serde(rename = "optional")]
    Optional {
        /// The bypassable item.
        item: Box<RailroadNode>,
    },
    /// An item that repeats at least once.
    #[serde(rename = "oneOrMore")]
    OneOrMore {
        /// The repeated item.
        item: Box<RailroadNode>,
        /// What sits on the return path (a separator), when anything.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        rep: Option<Box<RailroadNode>>,
    },
    /// An item that repeats any number of times, including none.
    #[serde(rename = "zeroOrMore")]
    ZeroOrMore {
        /// The repeated item.
        item: Box<RailroadNode>,
        /// What sits on the return path (a separator), when anything.
        #[serde(default, skip_serializing_if = "Option::is_none")]
        rep: Option<Box<RailroadNode>>,
    },
    /// A top-level wrapper, rendered like a sequence.
    #[serde(rename = "diagram")]
    Diagram {
        /// The items, top to bottom.
        items: Vec<RailroadNode>,
    },
}

/// A bare string is taken to be a terminal, the common case when
/// hand-building diagrams: `sequence(["(", "expr", ")"])`. This is the
/// TypeScript `norm` coercion of its `Item` union.
impl From<&str> for RailroadNode {
    fn from(text: &str) -> Self {
        terminal(text)
    }
}

impl From<String> for RailroadNode {
    fn from(text: String) -> Self {
        terminal(text)
    }
}

impl From<&RailroadNode> for RailroadNode {
    fn from(node: &RailroadNode) -> Self {
        node.clone()
    }
}

impl RailroadNode {
    /// The `kind` tag this node serializes with.
    pub fn kind(&self) -> &'static str {
        match self {
            RailroadNode::Terminal { .. } => "terminal",
            RailroadNode::NonTerminal { .. } => "nonterminal",
            RailroadNode::Comment { .. } => "comment",
            RailroadNode::Skip => "skip",
            RailroadNode::Seq { .. } => "seq",
            RailroadNode::Choice { .. } => "choice",
            RailroadNode::Optional { .. } => "optional",
            RailroadNode::OneOrMore { .. } => "oneOrMore",
            RailroadNode::ZeroOrMore { .. } => "zeroOrMore",
            RailroadNode::Diagram { .. } => "diagram",
        }
    }

    /// The `text` of a terminal, nonterminal or comment.
    pub fn text(&self) -> Option<&str> {
        match self {
            RailroadNode::Terminal { text }
            | RailroadNode::NonTerminal { text }
            | RailroadNode::Comment { text } => Some(text),
            _ => None,
        }
    }

    /// The `items` of a sequence, choice or diagram.
    pub fn items(&self) -> Option<&[RailroadNode]> {
        match self {
            RailroadNode::Seq { items }
            | RailroadNode::Choice { items }
            | RailroadNode::Diagram { items } => Some(items),
            _ => None,
        }
    }

    /// The `item` of an optional or a repetition.
    pub fn item(&self) -> Option<&RailroadNode> {
        match self {
            RailroadNode::Optional { item }
            | RailroadNode::OneOrMore { item, .. }
            | RailroadNode::ZeroOrMore { item, .. } => Some(item),
            _ => None,
        }
    }

    /// The `rep` of a repetition, when it has one.
    pub fn rep(&self) -> Option<&RailroadNode> {
        match self {
            RailroadNode::OneOrMore { rep, .. } | RailroadNode::ZeroOrMore { rep, .. } => {
                rep.as_deref()
            }
            _ => None,
        }
    }

    /// Decode a node from its JSON shape.
    ///
    /// An unknown `kind`, or a value that is not a node at all, is a
    /// [`RailroadError`]: that is where this port rejects what the
    /// TypeScript `norm` rejects at render time.
    pub fn from_json(text: &str) -> Result<RailroadNode, RailroadError> {
        serde_json::from_str(text).map_err(|error| RailroadError {
            message: format!("railroad: invalid diagram node: {error}"),
            node: serde_json::from_str(text).ok(),
        })
    }

    /// Encode this node as compact JSON.
    pub fn to_json(&self) -> String {
        serde_json::to_string(self).expect("a node always serializes")
    }
}

/// One entry of a token key: a token (or token set) label as it appears
/// in the diagram, and its human meaning.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct LegendEntry {
    /// The label as it appears in a terminal box.
    pub token: String,
    /// What the token means.
    pub meaning: String,
}

impl LegendEntry {
    /// Build an entry.
    pub fn new(token: impl Into<String>, meaning: impl Into<String>) -> Self {
        LegendEntry {
            token: token.into(),
            meaning: meaning.into(),
        }
    }
}

/// One whole grammar: an ordered rule map plus the entry rule. This is
/// the declarative artifact emitted as `grammar.railroad.json`.
///
/// `rules` keeps insertion order, so the order the grammar declared its
/// rules in survives a round trip through JSON, as it does in the
/// TypeScript object. The fields serialize in the TypeScript key order.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize, Default)]
pub struct GrammarModel {
    /// The entry rule.
    pub start: String,
    /// One node tree per rule, in declaration order.
    pub rules: IndexMap<String, RailroadNode>,
    /// Open-ended metadata; extraction sets `engine`.
    #[serde(default, skip_serializing_if = "IndexMap::is_empty")]
    pub meta: IndexMap<String, serde_json::Value>,
    /// Key for the named tokens (non-literal labels) that appear in the
    /// diagram.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub legend: Vec<LegendEntry>,
    /// Tokens the lexer silently skips (the IGNORE set): whitespace,
    /// comments and the like. They never appear in a rule, so they are
    /// reported separately.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub ignored: Vec<LegendEntry>,
}

impl GrammarModel {
    /// Decode a model from its JSON text.
    pub fn from_json(text: &str) -> Result<GrammarModel, RailroadError> {
        serde_json::from_str(text).map_err(|error| RailroadError {
            message: format!("railroad: invalid grammar model: {error}"),
            node: None,
        })
    }

    /// Encode this model as JSON indented by two spaces, the form the
    /// CLI writes to `grammar.railroad.json` (`JSON.stringify(model,
    /// null, 2)` in TypeScript).
    pub fn to_json_pretty(&self) -> String {
        serde_json::to_string_pretty(self).expect("a model always serializes")
    }

    /// Encode this model as compact JSON.
    pub fn to_json(&self) -> String {
        serde_json::to_string(self).expect("a model always serializes")
    }

    /// The rule names in rendering order: `start` first when it names a
    /// rule, then the rest in declaration order.
    pub fn rule_order(&self) -> Vec<&str> {
        let mut names: Vec<&str> = Vec::with_capacity(self.rules.len());
        if self.rules.contains_key(&self.start) {
            names.push(self.start.as_str());
        }
        names.extend(
            self.rules
                .keys()
                .map(String::as_str)
                .filter(|name| *name != self.start),
        );
        names
    }

    /// The `engine` entry of `meta`, when it is a string.
    pub fn engine(&self) -> Option<&str> {
        self.meta.get("engine").and_then(serde_json::Value::as_str)
    }
}

/// Raised on a malformed diagram model: a choice with no branches, or
/// JSON that does not decode to a node.
#[derive(Debug, Clone, PartialEq)]
pub struct RailroadError {
    /// What went wrong.
    pub message: String,
    /// The offending value, when there is one to show.
    pub node: Option<serde_json::Value>,
}

impl RailroadError {
    /// An error with a message and no node.
    pub fn new(message: impl Into<String>) -> Self {
        RailroadError {
            message: message.into(),
            node: None,
        }
    }
}

impl fmt::Display for RailroadError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(&self.message)
    }
}

impl std::error::Error for RailroadError {}

// ---- node constructors ---------------------------------------------

/// A terminal (literal or named token) node.
pub fn terminal(text: impl Into<String>) -> RailroadNode {
    RailroadNode::Terminal { text: text.into() }
}

/// A nonterminal (rule reference) node.
pub fn non_terminal(text: impl Into<String>) -> RailroadNode {
    RailroadNode::NonTerminal { text: text.into() }
}

/// An inline comment node.
pub fn comment(text: impl Into<String>) -> RailroadNode {
    RailroadNode::Comment { text: text.into() }
}

/// A bypass (empty) node.
pub fn skip() -> RailroadNode {
    RailroadNode::Skip
}

/// A sequence of nodes. A bare `&str` item is a terminal.
pub fn sequence<I>(items: impl IntoIterator<Item = I>) -> RailroadNode
where
    I: Into<RailroadNode>,
{
    RailroadNode::Seq {
        items: items.into_iter().map(Into::into).collect(),
    }
}

/// A choice of branches. A choice needs at least one branch; none is a
/// [`RailroadError`], as it is in TypeScript.
pub fn choice<I>(items: impl IntoIterator<Item = I>) -> Result<RailroadNode, RailroadError>
where
    I: Into<RailroadNode>,
{
    let items: Vec<RailroadNode> = items.into_iter().map(Into::into).collect();
    if items.is_empty() {
        return Err(RailroadError::new(
            "railroad: choice needs at least one branch",
        ));
    }
    Ok(RailroadNode::Choice { items })
}

/// An optional (bypassable) node.
pub fn optional(item: impl Into<RailroadNode>) -> RailroadNode {
    RailroadNode::Optional {
        item: Box::new(item.into()),
    }
}

/// A one-or-more repetition, with an optional node on the return path
/// (a separator, typically).
pub fn one_or_more(item: impl Into<RailroadNode>, rep: Option<RailroadNode>) -> RailroadNode {
    RailroadNode::OneOrMore {
        item: Box::new(item.into()),
        rep: rep.map(Box::new),
    }
}

/// A zero-or-more repetition (a bypassable [`one_or_more`]).
pub fn zero_or_more(item: impl Into<RailroadNode>, rep: Option<RailroadNode>) -> RailroadNode {
    RailroadNode::ZeroOrMore {
        item: Box::new(item.into()),
        rep: rep.map(Box::new),
    }
}

/// A top-level diagram wrapper.
pub fn diagram<I>(items: impl IntoIterator<Item = I>) -> RailroadNode
where
    I: Into<RailroadNode>,
{
    RailroadNode::Diagram {
        items: items.into_iter().map(Into::into).collect(),
    }
}

// ---- text emitter --------------------------------------------------

/// Compact EBNF-ish rendering: terminals quoted, a choice in `(a | b)`,
/// an optional in `[x]`, zero-or-more in `{x}`, one-or-more as `x+`.
pub fn to_text(node: &RailroadNode) -> String {
    match node {
        RailroadNode::Terminal { text } => json_string(text),
        RailroadNode::NonTerminal { text } => text.clone(),
        RailroadNode::Comment { text } => format!("/* {text} */"),
        RailroadNode::Skip => String::new(),
        RailroadNode::Seq { items } | RailroadNode::Diagram { items } => items
            .iter()
            .map(to_text)
            .filter(|text| !text.is_empty())
            .collect::<Vec<String>>()
            .join(" "),
        RailroadNode::Choice { items } => {
            format!(
                "({})",
                items
                    .iter()
                    .map(to_text)
                    .collect::<Vec<String>>()
                    .join(" | ")
            )
        }
        RailroadNode::Optional { item } => format!("[{}]", to_text(item)),
        RailroadNode::OneOrMore { item, rep } => format!("{}+{}", to_text(item), rep_text(rep)),
        RailroadNode::ZeroOrMore { item, rep } => {
            format!("{{{}}}{}", to_text(item), rep_text(rep))
        }
    }
}

fn rep_text(rep: &Option<Box<RailroadNode>>) -> String {
    match rep {
        Some(rep) => format!(" /* {} */", to_text(rep)),
        None => String::new(),
    }
}

// ---- structural helpers --------------------------------------------

/// Deep structural equality on nodes, the TypeScript `nodeEqual`. Here
/// it is the derived equality, named for parity with the other ports.
pub fn node_equal(a: &RailroadNode, b: &RailroadNode) -> bool {
    a == b
}

/// The JSON-encoded (double-quoted, escaped) form of `text`, what
/// `JSON.stringify` gives for a string.
pub(crate) fn json_string(text: &str) -> String {
    serde_json::to_string(text).expect("a string always serializes")
}
