/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Vertical-flow SVG renderer for railroad diagrams. Flow runs top to
//! bottom: sequences stack vertically, choice branches fan out side by
//! side, and optional and repetition rails run parallel on the side.
//! This biases the output tall and narrow, which suits laptop browsers
//! and phones.
//!
//! Each node measures to a layout (width, height, entry x, exit x, draw).
//! The rail enters the top edge at the entry x and leaves the bottom edge
//! at the exit x (both equal: every node is horizontally symmetric).
//! `draw(x, y)` emits SVG with the node's bounding box at top-left
//! `(x, y)`.
//!
//! [`model_to_svg`] stacks one titled, anchored sub-diagram per rule and
//! turns nonterminal boxes into `<a href="#rule">` links.
//!
//! This file mirrors `ts/src/svg.ts`.

use std::sync::Arc;

use crate::model::{GrammarModel, LegendEntry, RailroadNode};

// ---- geometry constants --------------------------------------------
const CHARW: f64 = 8.0;
const PADX: f64 = 10.0;
const BOXH: f64 = 26.0;
const MINW: f64 = 30.0;
/// Vertical gap between stacked items and split/merge stubs.
const VGAP: f64 = 18.0;
/// Horizontal gap between choice branches.
const HGAP: f64 = 26.0;
/// Loop rail inset.
const AR: f64 = 10.0;
/// Outer padding.
const PAD: f64 = 16.0;
/// Height reserved for a rule title.
const TITLE_H: f64 = 26.0;
const TRACK_GAP: f64 = 34.0;
/// Rail lead between a cap dot and the content.
const LEAD: f64 = 14.0;

/// Maps a nonterminal name to an `href`, or `None` to leave it unlinked.
pub type LinkFor = Arc<dyn Fn(&str) -> Option<String> + Send + Sync>;

/// Options for the SVG renderer.
#[derive(Clone, Default)]
pub struct SvgOptions {
    /// The link resolver for nonterminal boxes (whole-grammar linking).
    pub link_for: Option<LinkFor>,
}

impl SvgOptions {
    /// Link nonterminals through `link_for`.
    pub fn with_link_for(
        link_for: impl Fn(&str) -> Option<String> + Send + Sync + 'static,
    ) -> Self {
        SvgOptions {
            link_for: Some(Arc::new(link_for)),
        }
    }
}

impl std::fmt::Debug for SvgOptions {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter
            .debug_struct("SvgOptions")
            .field("link_for", &self.link_for.as_ref().map(|_| "<fn>"))
            .finish()
    }
}

type Draw = Box<dyn Fn(f64, f64) -> String>;

struct Layout {
    width: f64,
    height: f64,
    entry_x: f64,
    exit_x: f64,
    /// A bypass branch, which gets a tighter gap in a choice.
    is_skip: bool,
    draw: Draw,
}

// ---- primitives ----------------------------------------------------

fn esc(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for ch in s.chars() {
        match ch {
            '&' => out.push_str("&amp;"),
            '<' => out.push_str("&lt;"),
            '>' => out.push_str("&gt;"),
            '"' => out.push_str("&quot;"),
            _ => out.push(ch),
        }
    }
    out
}

/// A number the way JavaScript prints one: no decimal point on a whole
/// number, the shortest round-trip digits otherwise.
fn num(f: f64) -> String {
    if f == f.trunc() && f.is_finite() {
        format!("{}", f as i64)
    } else {
        format!("{f}")
    }
}

fn path(d: &str) -> String {
    format!(r#"<path class="rr-line" d="{d}"/>"#)
}

fn vline(x: f64, y1: f64, y2: f64) -> String {
    if y1 == y2 {
        String::new()
    } else {
        path(&format!("M{} {}V{}", num(x), num(y1), num(y2)))
    }
}

fn hline(x1: f64, x2: f64, y: f64) -> String {
    if x1 == x2 {
        String::new()
    } else {
        path(&format!("M{} {}H{}", num(x1), num(y), num(x2)))
    }
}

/// The length of `s` in UTF-16 units, the TypeScript `String.length`
/// every box width is computed from.
fn len16(s: &str) -> f64 {
    s.encode_utf16().count() as f64
}

fn cap(x: f64, y: f64) -> String {
    format!(
        r#"<circle class="rr-cap" cx="{}" cy="{}" r="3"/>"#,
        num(x),
        num(y)
    )
}

// ---- node layouts --------------------------------------------------

fn box_layout(text: &str, cls: &str, is_terminal: bool, href: Option<String>) -> Layout {
    let w = (len16(text) * CHARW + 2.0 * PADX).max(MINW);
    let h = BOXH;
    let text = text.to_string();
    let cls = cls.to_string();
    Layout {
        width: w,
        height: h,
        entry_x: w / 2.0,
        exit_x: w / 2.0,
        is_skip: false,
        draw: Box::new(move |x, y| {
            let r = if is_terminal { h / 2.0 } else { 4.0 };
            let inner = format!(
                r#"<rect class="{cls}" x="{}" y="{}" width="{}" height="{}" rx="{}" ry="{}"/><text class="rr-label" x="{}" y="{}">{}</text>"#,
                num(x),
                num(y),
                num(w),
                num(h),
                num(r),
                num(r),
                num(x + w / 2.0),
                num(y + h / 2.0),
                esc(&text)
            );
            match &href {
                Some(href) => format!(r#"<a href="{}">{inner}</a>"#, esc(href)),
                None => inner,
            }
        }),
    }
}

fn comment_layout(text: &str) -> Layout {
    let w = (len16(text) * CHARW + 2.0 * PADX).max(MINW);
    let text = text.to_string();
    Layout {
        width: w,
        height: BOXH,
        entry_x: w / 2.0,
        exit_x: w / 2.0,
        is_skip: false,
        draw: Box::new(move |x, y| {
            format!(
                r#"{}<text class="rr-comment" x="{}" y="{}">{}</text>"#,
                vline(x + w / 2.0, y, y + BOXH),
                num(x + w / 2.0),
                num(y + BOXH / 2.0),
                esc(&text)
            )
        }),
    }
}

fn skip_layout() -> Layout {
    let (w, h) = (16.0, 12.0);
    Layout {
        width: w,
        height: h,
        entry_x: w / 2.0,
        exit_x: w / 2.0,
        is_skip: true,
        draw: Box::new(move |x, y| vline(x + w / 2.0, y, y + h)),
    }
}

fn seq_layout(mut children: Vec<Layout>) -> Layout {
    if children.is_empty() {
        return skip_layout();
    }
    if children.len() == 1 {
        return children.remove(0);
    }
    let rail_x = children.iter().map(|c| c.entry_x).fold(0.0, f64::max);
    let offs: Vec<f64> = children.iter().map(|c| rail_x - c.entry_x).collect();
    let width = children
        .iter()
        .zip(&offs)
        .map(|(c, off)| off + c.width)
        .fold(0.0, f64::max);
    let height =
        children.iter().map(|c| c.height).sum::<f64>() + VGAP * (children.len() as f64 - 1.0);
    Layout {
        width,
        height,
        entry_x: rail_x,
        exit_x: rail_x,
        is_skip: false,
        draw: Box::new(move |x, y| {
            let mut out = String::new();
            let mut cy = y;
            for (i, c) in children.iter().enumerate() {
                if i > 0 {
                    out.push_str(&vline(x + rail_x, cy - VGAP, cy));
                }
                out.push_str(&(c.draw)(x + offs[i], cy));
                cy += c.height + VGAP;
            }
            out
        }),
    }
}

fn choice_layout(mut branches: Vec<Layout>) -> Layout {
    // A choice with no branches cannot come from a constructor or from
    // JSON, but the enum can be built by hand; it renders as a bypass
    // rather than indexing a branch that is not there.
    if branches.is_empty() {
        return skip_layout();
    }
    if branches.len() == 1 {
        return branches.remove(0);
    }
    let n = branches.len();
    let max_bh = branches.iter().map(|c| c.height).fold(0.0, f64::max);
    // Entry stub + fan-in + branches + fan-out + exit stub.
    let height = 4.0 * VGAP + max_bh;
    // Branch x offsets within the box, and the entry on the fan midpoint.
    // A bypass (skip) branch hugs its neighbour with a tighter gap.
    let mut bxs = Vec::with_capacity(n);
    let mut bx = 0.0;
    for (i, c) in branches.iter().enumerate() {
        bxs.push(bx);
        let gap = match branches.get(i + 1) {
            Some(next) if c.is_skip || next.is_skip => AR,
            _ => HGAP,
        };
        bx += c.width + gap;
    }
    let width = bxs[n - 1] + branches[n - 1].width;
    let rel_centers: Vec<f64> = branches
        .iter()
        .zip(&bxs)
        .map(|(c, bx)| bx + c.entry_x)
        .collect();
    let entry_x = (rel_centers[0] + rel_centers[n - 1]) / 2.0;
    Layout {
        width,
        height,
        entry_x,
        exit_x: entry_x,
        is_skip: false,
        draw: Box::new(move |x, y| {
            let split_y = y + VGAP;
            let branch_y = split_y + VGAP;
            let merge_y = branch_y + max_bh + VGAP;
            let lo = rel_centers
                .iter()
                .map(|c| x + c)
                .fold(x + entry_x, f64::min);
            let hi = rel_centers
                .iter()
                .map(|c| x + c)
                .fold(x + entry_x, f64::max);
            let mut out = String::new();
            out.push_str(&vline(x + entry_x, y, split_y)); // entry stub
            out.push_str(&hline(lo, hi, split_y)); // split rail
            out.push_str(&hline(lo, hi, merge_y)); // merge rail
            out.push_str(&vline(x + entry_x, merge_y, y + height)); // exit stub
            for (i, c) in branches.iter().enumerate() {
                let cx = x + bxs[i];
                let bex = cx + c.entry_x;
                out.push_str(&vline(bex, split_y, branch_y)); // fan-in drop
                out.push_str(&(c.draw)(cx, branch_y));
                out.push_str(&vline(bex, branch_y + c.height, merge_y)); // fan-out drop
            }
            out
        }),
    }
}

fn one_or_more_layout(item: Layout, rep: Option<Layout>) -> Layout {
    let rep_w = rep.as_ref().map_or(0.0, |r| r.width);
    // The x of the return rail, box-relative.
    let rail_x0 = item.width + AR + rep_w / 2.0;
    let width = item.width + 2.0 * AR + rep_w;
    let (top_gap, bot_gap) = (VGAP, VGAP);
    let height = top_gap + item.height + bot_gap;
    let entry_x = item.entry_x;
    Layout {
        width,
        height,
        entry_x,
        exit_x: item.exit_x,
        is_skip: false,
        draw: Box::new(move |x, y| {
            let main_x = x + entry_x;
            let item_y = y + top_gap;
            let rail_x = x + rail_x0;
            let loop_top_y = y + top_gap / 2.0;
            let loop_bottom_y = y + height - bot_gap / 2.0;
            let mut out = String::new();
            out.push_str(&vline(main_x, y, item_y));
            out.push_str(&(item.draw)(x, item_y));
            out.push_str(&vline(x + item.exit_x, item_y + item.height, y + height));
            out.push_str(&hline(main_x, rail_x, loop_bottom_y));
            out.push_str(&hline(main_x, rail_x, loop_top_y));
            match &rep {
                Some(rep) => {
                    let rep_h = rep.height;
                    let mid_y = (loop_top_y + loop_bottom_y) / 2.0;
                    let r_top = mid_y - rep_h / 2.0;
                    let r_bot = mid_y + rep_h / 2.0;
                    out.push_str(&vline(rail_x, r_bot, loop_bottom_y));
                    out.push_str(&vline(rail_x, loop_top_y, r_top));
                    out.push_str(&(rep.draw)(rail_x - rep.entry_x, r_top));
                }
                None => out.push_str(&vline(rail_x, loop_top_y, loop_bottom_y)),
            }
            out
        }),
    }
}

fn layout_node(node: &RailroadNode, opts: &SvgOptions) -> Layout {
    match node {
        RailroadNode::Terminal { text } => box_layout(text, "rr-term", true, None),
        RailroadNode::NonTerminal { text } => {
            let href = opts.link_for.as_ref().and_then(|link_for| link_for(text));
            box_layout(text, "rr-nonterm", false, href)
        }
        RailroadNode::Comment { text } => comment_layout(text),
        RailroadNode::Skip => skip_layout(),
        RailroadNode::Seq { items } | RailroadNode::Diagram { items } => {
            seq_layout(items.iter().map(|n| layout_node(n, opts)).collect())
        }
        RailroadNode::Choice { items } => {
            choice_layout(items.iter().map(|n| layout_node(n, opts)).collect())
        }
        RailroadNode::Optional { item } => {
            choice_layout(vec![layout_node(item, opts), skip_layout()])
        }
        RailroadNode::OneOrMore { item, rep } => one_or_more_layout(
            layout_node(item, opts),
            rep.as_deref().map(|r| layout_node(r, opts)),
        ),
        RailroadNode::ZeroOrMore { item, rep } => choice_layout(vec![
            one_or_more_layout(
                layout_node(item, opts),
                rep.as_deref().map(|r| layout_node(r, opts)),
            ),
            skip_layout(),
        ]),
    }
}

// ---- document assembly ---------------------------------------------

const STYLE: &str = concat!(
    "svg.railroad{background:#fff;font-family:monospace;font-size:13px}",
    ".rr-line{stroke:#334;stroke-width:2;fill:none}",
    ".rr-cap{fill:#334}",
    ".rr-term{fill:#e8f0ff;stroke:#334;stroke-width:2}",
    ".rr-nonterm{fill:#fff7e8;stroke:#334;stroke-width:2}",
    ".rr-label{fill:#111;text-anchor:middle;dominant-baseline:middle}",
    ".rr-comment{fill:#666;font-style:italic;text-anchor:middle;dominant-baseline:middle}",
    ".rr-title{fill:#113;font-weight:bold;font-size:15px}",
    ".rr-legend{fill:#333;font-size:12px}",
    ".rr-legend-tok{fill:#113;font-weight:bold}",
    "a:hover .rr-nonterm{fill:#ffe6b3;cursor:pointer}",
);

fn svg_doc(body: &str, w: f64, h: f64) -> String {
    let w = w.ceil() as i64;
    let h = h.ceil() as i64;
    format!(
        r#"<svg xmlns="http://www.w3.org/2000/svg" class="railroad" width="{w}" height="{h}" viewBox="0 0 {w} {h}"><style>{STYLE}</style><g>{body}</g></svg>"#
    )
}

/// Draw a layout with entry and exit cap dots, separated from the
/// content by a short rail lead so the dots never sit on top of a box.
fn with_caps(layout: &Layout, x: f64, top: f64) -> String {
    let ct = top + LEAD;
    let cb = ct + layout.height;
    let ex = x + layout.entry_x;
    let xx = x + layout.exit_x;
    format!(
        "{}{}{}{}{}",
        cap(ex, top),
        vline(ex, top, ct),
        (layout.draw)(x, ct),
        vline(xx, cb, cb + LEAD),
        cap(xx, cb + LEAD)
    )
}

/// Render a single node, wrapped in its own SVG document.
pub fn render_node_svg(node: &RailroadNode, opts: &SvgOptions) -> String {
    let layout = layout_node(node, opts);
    svg_doc(
        &with_caps(&layout, PAD, PAD),
        layout.width + 2.0 * PAD,
        layout.height + 2.0 * LEAD + 2.0 * PAD,
    )
}

/// Render a whole grammar: a vertical stack of titled, anchored rule
/// tracks, in two newspaper-style columns balanced by height, with
/// nonterminal boxes linking to the referenced rule's track, then the
/// token key and the ignored-token key when the model carries them.
pub fn model_to_svg(model: &GrammarModel) -> String {
    let rule_names: Vec<String> = model.rules.keys().cloned().collect();
    let opts = SvgOptions::with_link_for(move |name| {
        rule_names
            .iter()
            .any(|rule| rule == name)
            .then(|| format!("#{name}"))
    });

    // Measure each rule track (title + capped diagram).
    struct Track<'a> {
        name: &'a str,
        layout: Layout,
        h: f64,
    }
    let tracks: Vec<Track> = model
        .rule_order()
        .into_iter()
        .map(|name| {
            let layout = layout_node(&model.rules[name], &opts);
            let h = TITLE_H + layout.height + 2.0 * LEAD;
            Track { name, layout, h }
        })
        .collect();

    // Lay the tracks out in two columns, filled top to bottom and
    // balanced by height.
    let two_col = tracks.len() > 1;
    let total: f64 = tracks.iter().map(|t| t.h + TRACK_GAP).sum();
    let mut cols: [Vec<&Track>; 2] = [Vec::new(), Vec::new()];
    let mut acc = 0.0;
    let mut ci = 0;
    for t in &tracks {
        cols[ci].push(t);
        acc += t.h + TRACK_GAP;
        if two_col && ci == 0 && acc >= total / 2.0 {
            ci = 1;
        }
    }

    const COLGAP: f64 = 48.0;
    let mut x = PAD;
    let mut page_h = 0.0_f64;
    let mut body = String::new();
    for col in &cols {
        if col.is_empty() {
            continue;
        }
        let mut y = PAD;
        let mut col_w = 0.0_f64;
        for t in col {
            let dy = y + TITLE_H;
            body.push_str(&format!(r#"<g id="{}">"#, esc(t.name)));
            body.push_str(&format!(
                r#"<text class="rr-title" x="{}" y="{}">{}</text>"#,
                num(x),
                num(y + 15.0),
                esc(t.name)
            ));
            body.push_str(&with_caps(&t.layout, x, dy));
            body.push_str("</g>");
            col_w = col_w.max(t.layout.width);
            y = dy + t.layout.height + 2.0 * LEAD + TRACK_GAP;
        }
        page_h = page_h.max(y - TRACK_GAP);
        x += col_w + COLGAP;
    }
    let mut page_w = x - COLGAP + PAD;

    // Token key and ignored-token key below the rule tracks.
    let mut render_key = |title: &str, entries: &[LegendEntry]| {
        let mut ly = page_h + TRACK_GAP;
        body.push_str(&format!(
            r#"<text class="rr-title" x="{}" y="{}">{}</text>"#,
            num(PAD),
            num(ly + 15.0),
            esc(title)
        ));
        ly += TITLE_H;
        for e in entries {
            body.push_str(&format!(
                r#"<text class="rr-legend" x="{}" y="{}"><tspan class="rr-legend-tok">{}</tspan>  —  {}</text>"#,
                num(PAD),
                num(ly + 11.0),
                esc(&e.token),
                esc(&e.meaning)
            ));
            let chars = len16(&e.token) + len16(&e.meaning) + 5.0;
            page_w = page_w.max(PAD + chars * 7.5 + PAD);
            ly += 18.0;
        }
        page_h = ly;
    };
    if !model.legend.is_empty() {
        render_key("Tokens", &model.legend);
    }
    // Tokens the lexer silently skips never appear in a rule, so they
    // are listed separately from the diagram tokens.
    if !model.ignored.is_empty() {
        render_key("Ignored tokens", &model.ignored);
    }
    svg_doc(&body, page_w, page_h + PAD)
}
