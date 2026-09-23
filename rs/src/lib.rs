/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Railroad (syntax) diagram renderer for the `tabnas` parser: the Rust
//! port of `@tabnas/railroad`.
//!
//! It does not parse anything itself. It introspects a live [`Tabnas`]
//! instance that already has a grammar installed and emits three
//! artifacts from that grammar:
//!
//! - a declarative, JSON-serializable [`GrammarModel`] (the interchange
//!   format: one node tree per rule),
//! - a vertical-flow SVG (one anchored, linked track per rule), and
//! - a vertical ASCII diagram (Unicode box drawing, or plain `| - +`).
//!
//! ```
//! use tabnas_railroad::{extract_grammar, model_to_ascii, AsciiOptions, ExtractOptions};
//!
//! let parser = tabnas_json::make();
//! let model = extract_grammar(&parser, &ExtractOptions::default());
//! assert_eq!(model.start, "val");
//! assert_eq!(model.rules.len(), 5);
//!
//! let ascii = model_to_ascii(&model, &AsciiOptions::default());
//! assert!(ascii.starts_with("val:\n"));
//! ```
//!
//! The extraction and rendering logic lives in `extract`, `svg` and
//! `ascii`; this file is the plugin wiring plus the bare re-exports for
//! instance-free use. Because the model is pure data, the SVG and ASCII
//! are fully reproducible from the JSON alone: no renderer reads
//! anything off the live instance.

pub mod ascii;
#[cfg(feature = "cli")]
pub mod cli;
pub mod extract;
pub mod model;
pub mod svg;

use tabnas::{Plugin, PluginError, Tabnas};

pub use ascii::{model_to_ascii, render_node_ascii, AsciiOptions};
pub use extract::{extract_grammar, ExtractOptions};
pub use model::{
    choice, comment, diagram, node_equal, non_terminal, one_or_more, optional, sequence, skip,
    terminal, to_text, zero_or_more, GrammarModel, LegendEntry, RailroadError, RailroadNode,
};
pub use svg::{model_to_svg, render_node_svg, LinkFor, SvgOptions};

/// The README's Rust examples run as doctests, so a stale one fails the
/// gate rather than misleading the reader. Its `text`, `toml` and `bash`
/// fences are skipped; rustdoc runs only the `rust` ones.
#[cfg(doctest)]
#[doc = include_str!("../README.md")]
mod readme_examples {}

/// This crate's version. It MUST equal `ts/package.json` "version": the
/// release orchestrator rewrites both, and `tests/version_test.rs` fails
/// the build if they drift. Mirrors `VERSION` in `ts/src/railroad.ts`
/// and `const VERSION` in `go/model.go`.
pub const VERSION: &str = "0.3.7";

/// The key under which the plugin marks the instance (retrieve the mark
/// with `parser.decoration::<RailroadDecoration>(DECORATION_NAME)`).
pub const DECORATION_NAME: &str = "railroad";

/// The mark the plugin leaves on an instance. TypeScript decorates the
/// instance with a callable API bound to it; a Rust decoration cannot
/// hold a reference to the instance it sits on, so the mark is a unit
/// and [`of`] binds the API to any instance on demand.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RailroadDecoration;

/// The plugin, for `parser.use_plugin(tabnas_railroad::plugin(), None)`.
///
/// Installing it marks the instance (see [`installed`]). Everything else
/// is lazy: [`of`] re-reads the instance's current grammar on every
/// call, so plugin install order does not matter.
pub fn plugin() -> Plugin {
    Plugin::new("railroad", |parser: &mut Tabnas, _options| {
        parser.decorate(DECORATION_NAME, RailroadDecoration);
        Ok(())
    })
}

/// Install the plugin on `parser`.
///
/// ```
/// let mut parser = tabnas_json::make();
/// tabnas_railroad::railroad(&mut parser)?;
/// assert!(tabnas_railroad::installed(&parser));
/// # Ok::<(), tabnas::PluginError>(())
/// ```
pub fn railroad(parser: &mut Tabnas) -> Result<(), PluginError> {
    parser.use_plugin(plugin(), None).map(|_| ())
}

/// Whether [`plugin`] has been installed on `parser` (or on an instance
/// it was derived from).
pub fn installed(parser: &Tabnas) -> bool {
    parser
        .decoration::<RailroadDecoration>(DECORATION_NAME)
        .is_some()
}

/// The API bound to one instance: the Rust analog of the TypeScript
/// `tn.railroad` callable, and of the Go `RailroadApi`.
#[derive(Clone, Copy)]
pub struct RailroadApi<'a> {
    parser: &'a Tabnas,
}

impl std::fmt::Debug for RailroadApi<'_> {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("RailroadApi")
            .field("start", &self.parser.options.rule.start)
            .field("rules", &self.parser.rule_names())
            .finish()
    }
}

/// Bind the API to `parser`, whether or not the plugin is installed.
pub fn of(parser: &Tabnas) -> RailroadApi<'_> {
    RailroadApi { parser }
}

impl<'a> RailroadApi<'a> {
    /// The instance this API reads.
    pub fn parser(&self) -> &'a Tabnas {
        self.parser
    }

    /// Introspect this instance's grammar and return its model.
    pub fn extract(&self, opts: &ExtractOptions) -> GrammarModel {
        extract_grammar(self.parser, opts)
    }

    /// The model with default extraction options (the TypeScript
    /// `tn.railroad()` / `tn.railroad.toJson()`).
    pub fn to_json(&self) -> GrammarModel {
        self.extract(&ExtractOptions::default())
    }

    /// Extract this instance's grammar and render it to SVG.
    pub fn to_svg(&self) -> String {
        model_to_svg(&self.to_json())
    }

    /// Extract this instance's grammar and render it to ASCII.
    pub fn to_ascii(&self, opts: &AsciiOptions) -> String {
        model_to_ascii(&self.to_json(), opts)
    }

    /// Render a single node to a standalone SVG.
    pub fn render_node(&self, node: &RailroadNode, opts: &SvgOptions) -> String {
        render_node_svg(node, opts)
    }

    /// Render a single node to an ASCII block.
    pub fn render_node_ascii(&self, node: &RailroadNode, opts: &AsciiOptions) -> String {
        render_node_ascii(node, opts)
    }

    /// Render a single node to compact EBNF text.
    pub fn render_node_text(&self, node: &RailroadNode) -> String {
        to_text(node)
    }
}
