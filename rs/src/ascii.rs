/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

//! Vertical-flow ASCII renderer for railroad diagrams. Same flow model as
//! the SVG renderer (sequences stack downward, choices fan sideways,
//! loops return on the right), painted onto a character grid.
//!
//! Rails are tracked as direction bits per cell (up, down, left, right)
//! so junctions resolve automatically to the right box-drawing glyph.
//! Boxes draw literal corners and sides. [`AsciiOptions::ascii`] (the
//! CLI `--ascii-plain`) swaps the Unicode glyph set for plain `| - +`.
//!
//! This file mirrors `ts/src/ascii.ts`.

use crate::model::{json_string, GrammarModel, LegendEntry, RailroadNode};

/// Options for the ASCII renderer.
#[derive(Debug, Clone, Copy, Default, PartialEq, Eq)]
pub struct AsciiOptions {
    /// Pure-ASCII glyphs (`| - +`) instead of Unicode box drawing. The
    /// TypeScript spelling is `{ ascii: true }`; the Go field is `Plain`.
    pub ascii: bool,
}

impl AsciiOptions {
    /// The plain `| - +` glyph set.
    pub fn plain() -> Self {
        AsciiOptions { ascii: true }
    }
}

const U: u8 = 1;
const D: u8 = 2;
const L: u8 = 4;
const R: u8 = 8;

fn glyph_for(bits: u8, plain: bool) -> char {
    if bits == 0 {
        return ' ';
    }
    if plain {
        let vertical = bits & 3 != 0;
        let horizontal = bits & 12 != 0;
        return match (vertical, horizontal) {
            (true, true) => '+',
            (true, false) => '|',
            _ => '-',
        };
    }
    match bits {
        1..=3 => '│',
        12 | 4 | 8 => '─',
        10 => '┌',
        6 => '┐',
        9 => '└',
        5 => '┘',
        11 => '├',
        7 => '┤',
        14 => '┬',
        13 => '┴',
        15 => '┼',
        _ => ' ',
    }
}

#[derive(Clone, Copy)]
struct Glyphs {
    tl: char,
    tr: char,
    bl: char,
    br: char,
    rtl: char,
    rtr: char,
    rbl: char,
    rbr: char,
    v: char,
}

fn glyphs(plain: bool) -> Glyphs {
    if plain {
        Glyphs {
            tl: '+',
            tr: '+',
            bl: '+',
            br: '+',
            rtl: '+',
            rtr: '+',
            rbl: '+',
            rbr: '+',
            v: '|',
        }
    } else {
        Glyphs {
            tl: '┌',
            tr: '┐',
            bl: '└',
            br: '┘',
            rtl: '╭',
            rtr: '╮',
            rbl: '╰',
            rbr: '╯',
            v: '│',
        }
    }
}

/// A literal cell: a character, or the trailing half of a character
/// that JavaScript counts as two UTF-16 units. The character is printed
/// by the cell before it; the trailing half prints nothing, so the row
/// renders to the same string the two-cell JavaScript canvas gives.
#[derive(Clone, Copy)]
enum Lit {
    Char(char),
    Trail,
}

/// The character grid: per-cell direction bits resolve to junction
/// glyphs, and a literal cell overrides them.
#[derive(Default)]
struct Canvas {
    bits: Vec<Vec<u8>>,
    lit: Vec<Vec<Option<Lit>>>,
}

impl Canvas {
    fn grow(&mut self, r: usize, c: usize) {
        while self.bits.len() <= r {
            self.bits.push(Vec::new());
            self.lit.push(Vec::new());
        }
        let row = &mut self.bits[r];
        let lrow = &mut self.lit[r];
        while row.len() <= c {
            row.push(0);
            lrow.push(None);
        }
    }

    fn line(&mut self, r: usize, c: usize, mask: u8) {
        self.grow(r, c);
        self.bits[r][c] |= mask;
    }

    fn put(&mut self, r: usize, c: usize, ch: char) {
        self.grow(r, c);
        self.lit[r][c] = Some(Lit::Char(ch));
    }

    /// Write `s` from column `c`, one cell per UTF-16 unit: a character
    /// outside the Basic Multilingual Plane takes two cells, as it does
    /// in TypeScript, so every width and rail column agrees.
    fn text(&mut self, r: usize, c: usize, s: &str) {
        let mut col = c;
        for ch in s.chars() {
            self.put(r, col, ch);
            col += 1;
            if ch.len_utf16() == 2 {
                self.grow(r, col);
                self.lit[r][col] = Some(Lit::Trail);
                col += 1;
            }
        }
    }

    fn vline(&mut self, r1: usize, r2: usize, c: usize) {
        let (r1, r2) = if r1 > r2 { (r2, r1) } else { (r1, r2) };
        for r in r1..=r2 {
            let mask = if r > r1 { U } else { 0 } | if r < r2 { D } else { 0 };
            self.line(r, c, mask);
        }
    }

    fn hline(&mut self, c1: usize, c2: usize, r: usize) {
        let (c1, c2) = if c1 > c2 { (c2, c1) } else { (c1, c2) };
        for c in c1..=c2 {
            let mask = if c > c1 { L } else { 0 } | if c < c2 { R } else { 0 };
            self.line(r, c, mask);
        }
    }

    fn render(&self, plain: bool) -> String {
        self.bits
            .iter()
            .enumerate()
            .map(|(r, row)| {
                let mut line = String::with_capacity(row.len());
                for (c, bits) in row.iter().enumerate() {
                    match self.lit[r][c] {
                        Some(Lit::Char(ch)) => line.push(ch),
                        Some(Lit::Trail) => {}
                        None => line.push(glyph_for(*bits, plain)),
                    }
                }
                js_trim_end(&line).to_string()
            })
            .collect::<Vec<String>>()
            .join("\n")
    }
}

/// Whether `ch` is whitespace to a JavaScript `\s`: the ECMAScript
/// WhiteSpace and LineTerminator sets. This is not Rust's `White_Space`
/// (which has NEXT LINE and lacks the byte order mark), and the renderer
/// trims a row's end with this so a rail label ending in such a character
/// trims exactly as it does in TypeScript.
fn is_js_whitespace(ch: char) -> bool {
    matches!(
        ch,
        '\t' | '\n' | '\u{0B}' | '\u{0C}' | '\r' | ' ' | '\u{A0}' | '\u{1680}' | '\u{2000}'
            ..='\u{200A}'
                | '\u{2028}'
                | '\u{2029}'
                | '\u{202F}'
                | '\u{205F}'
                | '\u{3000}'
                | '\u{FEFF}'
    )
}

/// `line` without its trailing JavaScript whitespace: the
/// `replace(/\s+$/, '')` of the TypeScript canvas.
fn js_trim_end(line: &str) -> &str {
    line.trim_end_matches(is_js_whitespace)
}

// ---- measure model -------------------------------------------------

type Paint = Box<dyn Fn(&mut Canvas, usize, usize)>;

/// A measured node: its size, where the rail enters and leaves, and how
/// to paint it at a position.
struct Measure {
    cols: usize,
    rows: usize,
    entry_col: usize,
    exit_col: usize,
    paint: Paint,
}

/// Vertical gap rows between stacked items.
const VG: usize = 1;
/// Horizontal gap cols between choice branches.
const HG: usize = 3;

/// The width of `s` in cells: its length in UTF-16 units, which is what
/// the TypeScript `String.length` measures, so a label outside the Basic
/// Multilingual Plane is two cells wide in both.
fn width(s: &str) -> usize {
    s.encode_utf16().count()
}

fn box_m(text: &str, is_terminal: bool, g: Glyphs) -> Measure {
    let inner = format!(" {text} ");
    let w = width(&inner) + 2;
    let mid = w / 2;
    Measure {
        cols: w,
        rows: 3,
        entry_col: mid,
        exit_col: mid,
        paint: Box::new(move |cv, x, y| {
            let (tl, tr, bl, br) = if is_terminal {
                (g.rtl, g.rtr, g.rbl, g.rbr)
            } else {
                (g.tl, g.tr, g.bl, g.br)
            };
            cv.put(y, x, tl);
            cv.put(y, x + w - 1, tr);
            cv.hline(x + 1, x + w - 2, y);
            cv.put(y + 1, x, g.v);
            cv.put(y + 1, x + w - 1, g.v);
            cv.text(y + 1, x + 1, &inner);
            cv.put(y + 2, x, bl);
            cv.put(y + 2, x + w - 1, br);
            cv.hline(x + 1, x + w - 2, y + 2);
        }),
    }
}

fn comment_m(text: &str) -> Measure {
    let s = format!("/* {text} */");
    let w = width(&s);
    Measure {
        cols: w,
        rows: 1,
        entry_col: w / 2,
        exit_col: w / 2,
        paint: Box::new(move |cv, x, y| cv.text(y, x, &s)),
    }
}

fn skip_m() -> Measure {
    Measure {
        cols: 1,
        rows: 1,
        entry_col: 0,
        exit_col: 0,
        paint: Box::new(|cv, x, y| cv.line(y, x, U | D)),
    }
}

fn seq_m(mut children: Vec<Measure>) -> Measure {
    if children.is_empty() {
        return skip_m();
    }
    if children.len() == 1 {
        return children.remove(0);
    }
    let rail_col = children.iter().map(|c| c.entry_col).max().unwrap_or(0);
    let offs: Vec<usize> = children.iter().map(|c| rail_col - c.entry_col).collect();
    let cols = children
        .iter()
        .zip(&offs)
        .map(|(c, off)| off + c.cols)
        .max()
        .unwrap_or(0);
    let rows = children.iter().map(|c| c.rows).sum::<usize>() + VG * (children.len() - 1);
    Measure {
        cols,
        rows,
        entry_col: rail_col,
        exit_col: rail_col,
        paint: Box::new(move |cv, x, y| {
            let mut cy = y;
            for (i, c) in children.iter().enumerate() {
                if i > 0 {
                    cv.vline(cy - VG - 1, cy, x + rail_col);
                }
                (c.paint)(cv, x + offs[i], cy);
                cy += c.rows + VG;
            }
        }),
    }
}

fn choice_m(mut branches: Vec<Measure>) -> Measure {
    // A choice with no branches cannot come from a constructor or from
    // JSON, but the enum can be built by hand; it renders as a bypass
    // rather than indexing a branch that is not there.
    if branches.is_empty() {
        return skip_m();
    }
    if branches.len() == 1 {
        return branches.remove(0);
    }
    let n = branches.len();
    let cols = branches.iter().map(|c| c.cols).sum::<usize>() + HG * (n - 1);
    let max_r = branches.iter().map(|c| c.rows).max().unwrap_or(0);
    let rows = 1 + max_r + 1;
    let mut bxs = Vec::with_capacity(n);
    let mut bx = 0;
    for c in &branches {
        bxs.push(bx);
        bx += c.cols + HG;
    }
    let rel_centers: Vec<usize> = branches
        .iter()
        .zip(&bxs)
        .map(|(c, bx)| bx + c.entry_col)
        .collect();
    // `Math.round` of a half rounds up, which for these non-negative
    // integers is the ceiling of the midpoint.
    let entry_col = (rel_centers[0] + rel_centers[n - 1]).div_ceil(2);
    Measure {
        cols,
        rows,
        entry_col,
        exit_col: entry_col,
        paint: Box::new(move |cv, x, y| {
            let split_row = y;
            let branch_top = y + 1;
            let merge_row = y + 1 + max_r;
            let centers: Vec<usize> = rel_centers.iter().map(|c| x + c).collect();
            let lo = centers.iter().copied().fold(x + entry_col, usize::min);
            let hi = centers.iter().copied().fold(x + entry_col, usize::max);
            cv.hline(lo, hi, split_row);
            cv.hline(lo, hi, merge_row);
            for (i, c) in branches.iter().enumerate() {
                let cx = x + bxs[i];
                let ce = cx + c.entry_col;
                cv.vline(split_row, branch_top, ce);
                (c.paint)(cv, cx, branch_top);
                cv.vline(branch_top + c.rows - 1, merge_row, cx + c.exit_col);
            }
        }),
    }
}

fn one_or_more_m(item: Measure, rep_label: String) -> Measure {
    let rail_gap = if rep_label.is_empty() {
        0
    } else {
        width(&rep_label) + 1
    };
    let cols = item.cols + 1 + rail_gap;
    let rows = item.rows + 2;
    // One column right of the item block.
    let rail_col = item.cols;
    Measure {
        cols,
        rows,
        entry_col: item.entry_col,
        exit_col: item.exit_col,
        paint: Box::new(move |cv, x, y| {
            let top_row = y;
            let item_top = y + 1;
            // The last item row.
            let item_bot = y + item.rows;
            let bot_row = y + item.rows + 1;
            (item.paint)(cv, x, item_top);
            cv.vline(top_row, item_top, x + item.entry_col);
            cv.vline(item_bot, bot_row, x + item.exit_col);
            let rc = x + rail_col;
            cv.hline(x + item.entry_col, rc, top_row);
            cv.hline(x + item.exit_col, rc, bot_row);
            cv.vline(top_row, bot_row, rc);
            if !rep_label.is_empty() {
                cv.text(y + rows / 2, rc + 1, &rep_label);
            }
        }),
    }
}

fn label_of(node: Option<&RailroadNode>) -> String {
    match node {
        None => String::new(),
        Some(RailroadNode::Terminal { text }) | Some(RailroadNode::NonTerminal { text }) => {
            text.clone()
        }
        Some(_) => "+".to_string(),
    }
}

fn measure(node: &RailroadNode, g: Glyphs) -> Measure {
    match node {
        RailroadNode::Terminal { text } => box_m(&json_string(text), true, g),
        RailroadNode::NonTerminal { text } => box_m(text, false, g),
        RailroadNode::Comment { text } => comment_m(text),
        RailroadNode::Skip => skip_m(),
        RailroadNode::Seq { items } | RailroadNode::Diagram { items } => {
            seq_m(items.iter().map(|n| measure(n, g)).collect())
        }
        RailroadNode::Choice { items } => choice_m(items.iter().map(|n| measure(n, g)).collect()),
        RailroadNode::Optional { item } => choice_m(vec![measure(item, g), skip_m()]),
        RailroadNode::OneOrMore { item, rep } => {
            one_or_more_m(measure(item, g), label_of(rep.as_deref()))
        }
        RailroadNode::ZeroOrMore { item, rep } => choice_m(vec![
            one_or_more_m(measure(item, g), label_of(rep.as_deref())),
            skip_m(),
        ]),
    }
}

/// Paint a measured node with its top and bottom rail caps, joined into
/// the node's entry and exit cells.
fn paint_block(m: &Measure, plain: bool) -> String {
    let mut cv = Canvas::default();
    cv.line(0, m.entry_col, D);
    (m.paint)(&mut cv, 0, 1);
    cv.line(1, m.entry_col, U);
    cv.line(m.rows, m.exit_col, D);
    cv.line(m.rows + 1, m.exit_col, U);
    cv.render(plain)
}

/// Render a single node to an ASCII block.
pub fn render_node_ascii(node: &RailroadNode, opts: &AsciiOptions) -> String {
    let plain = opts.ascii;
    let m = measure(node, glyphs(plain));
    paint_block(&m, plain)
}

/// Render a whole grammar: each rule as a titled vertical block, then
/// the token key and the ignored-token key when the model carries them.
pub fn model_to_ascii(model: &GrammarModel, opts: &AsciiOptions) -> String {
    let plain = opts.ascii;
    let g = glyphs(plain);
    let mut blocks: Vec<String> = Vec::new();
    for name in model.rule_order() {
        let m = measure(&model.rules[name], g);
        blocks.push(format!("{name}:\n{}", paint_block(&m, plain)));
    }
    if !model.legend.is_empty() {
        blocks.push(key_block("Tokens", &model.legend));
    }
    // Tokens the lexer silently skips (the IGNORE set) never appear in a
    // rule, so they get their own key.
    if !model.ignored.is_empty() {
        blocks.push(key_block("Ignored tokens", &model.ignored));
    }
    blocks.join("\n\n")
}

fn key_block(title: &str, entries: &[LegendEntry]) -> String {
    let w = entries.iter().map(|e| width(&e.token)).max().unwrap_or(0);
    let lines: Vec<String> = entries
        .iter()
        .map(|e| {
            let pad = " ".repeat(w - width(&e.token));
            format!("  {}{pad} = {}", e.token, e.meaning)
        })
        .collect();
    format!("{title}:\n{}", lines.join("\n"))
}
