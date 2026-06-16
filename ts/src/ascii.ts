/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  ascii.ts
 *  Vertical-flow ASCII renderer for railroad diagrams. Same flow model as
 *  the SVG renderer (sequences stack downward, choices fan sideways, loops
 *  return on the right), painted onto a character grid.
 *
 *  Rails are tracked as direction bits per cell (Up/Down/Left/Right) so
 *  junctions resolve automatically to the right box-drawing glyph. Boxes
 *  draw literal corners/sides. `opts.ascii` (or the CLI `--ascii-plain`)
 *  swaps the Unicode glyph set for plain `| - +`.
 */

import { RailroadNode, GrammarModel, Item, norm, RailroadError } from './model'


export type AsciiOptions = {
  ascii?: boolean   // true => pure-ASCII glyphs (| - +) instead of Unicode
}

const U = 1, D = 2, L = 4, R = 8

const GLYPH: { [bits: number]: string } = {
  3: '│', 12: '─', 10: '┌', 6: '┐', 9: '└', 5: '┘',
  11: '├', 7: '┤', 14: '┬', 13: '┴', 15: '┼',
  1: '│', 2: '│', 4: '─', 8: '─',
}
function glyphFor(bits: number, plain: boolean): string {
  if (0 === bits) return ' '
  if (plain) {
    const v = bits & 3, h = bits & 12
    if (v && h) return '+'
    return v ? '|' : '-'
  }
  return GLYPH[bits] || ' '
}

function glyphs(plain: boolean) {
  return plain
    ? { tl: '+', tr: '+', bl: '+', br: '+', rtl: '+', rtr: '+', rbl: '+', rbr: '+', v: '|' }
    : { tl: '┌', tr: '┐', bl: '└', br: '┘', rtl: '╭', rtr: '╮', rbl: '╰', rbr: '╯', v: '│' }
}


class Canvas {
  private bits: number[][] = []
  private lit: (string | null)[][] = []
  private grow(r: number, c: number) {
    while (this.bits.length <= r) { this.bits.push([]); this.lit.push([]) }
    const row = this.bits[r], lrow = this.lit[r]
    while (row.length <= c) { row.push(0); lrow.push(null) }
  }
  line(r: number, c: number, mask: number) { this.grow(r, c); this.bits[r][c] |= mask }
  put(r: number, c: number, ch: string) { this.grow(r, c); this.lit[r][c] = ch }
  text(r: number, c: number, s: string) { for (let i = 0; i < s.length; i++) this.put(r, c + i, s[i]) }
  vline(r1: number, r2: number, c: number) {
    if (r1 > r2) { const t = r1; r1 = r2; r2 = t }
    for (let r = r1; r <= r2; r++) this.line(r, c, (r > r1 ? U : 0) | (r < r2 ? D : 0))
  }
  hline(c1: number, c2: number, r: number) {
    if (c1 > c2) { const t = c1; c1 = c2; c2 = t }
    for (let c = c1; c <= c2; c++) this.line(r, c, (c > c1 ? L : 0) | (c < c2 ? R : 0))
  }
  render(plain: boolean): string {
    return this.bits.map((row, r) => {
      let line = ''
      for (let c = 0; c < row.length; c++) {
        const lc = this.lit[r][c]
        line += null != lc ? lc : glyphFor(row[c], plain)
      }
      return line.replace(/\s+$/, '')
    }).join('\n')
  }
}


// ---- measure model -------------------------------------------------
type M = {
  cols: number
  rows: number
  entryCol: number
  exitCol: number
  paint: (cv: Canvas, x: number, y: number) => void
}

const VG = 1   // vertical gap rows between stacked items
const HG = 3   // horizontal gap cols between choice branches


function boxM(text: string, terminal: boolean, G: ReturnType<typeof glyphs>): M {
  const inner = ' ' + text + ' '
  const w = inner.length + 2
  const mid = Math.floor(w / 2)
  return {
    cols: w, rows: 3, entryCol: mid, exitCol: mid,
    paint(cv, x, y) {
      cv.put(y, x, terminal ? G.rtl : G.tl)
      cv.put(y, x + w - 1, terminal ? G.rtr : G.tr)
      cv.hline(x + 1, x + w - 2, y)
      cv.put(y + 1, x, G.v)
      cv.put(y + 1, x + w - 1, G.v)
      cv.text(y + 1, x + 1, inner)
      cv.put(y + 2, x, terminal ? G.rbl : G.bl)
      cv.put(y + 2, x + w - 1, terminal ? G.rbr : G.br)
      cv.hline(x + 1, x + w - 2, y + 2)
    },
  }
}

function commentM(text: string): M {
  const s = '/* ' + text + ' */'
  return {
    cols: s.length, rows: 1, entryCol: Math.floor(s.length / 2), exitCol: Math.floor(s.length / 2),
    paint(cv, x, y) { cv.text(y, x, s) },
  }
}

function skipM(): M {
  return { cols: 1, rows: 1, entryCol: 0, exitCol: 0, paint(cv, x, y) { cv.line(y, x, U | D) } }
}

function seqM(children: M[]): M {
  if (0 === children.length) return skipM()
  if (1 === children.length) return children[0]
  const railCol = Math.max(...children.map((c) => c.entryCol))
  const offs = children.map((c) => railCol - c.entryCol)
  const cols = children.reduce((m, c, i) => Math.max(m, offs[i] + c.cols), 0)
  const rows = children.reduce((a, c) => a + c.rows, 0) + VG * (children.length - 1)
  return {
    cols, rows, entryCol: railCol, exitCol: railCol,
    paint(cv, x, y) {
      let cy = y
      children.forEach((c, i) => {
        if (i > 0) { cv.vline(cy - VG - 1, cy, x + railCol) }
        c.paint(cv, x + offs[i], cy)
        cy += c.rows + VG
      })
    },
  }
}

function choiceM(branches: M[]): M {
  if (1 === branches.length) return branches[0]
  const n = branches.length
  const cols = branches.reduce((a, c) => a + c.cols, 0) + HG * (n - 1)
  const maxR = Math.max(...branches.map((c) => c.rows))
  const rows = 1 + maxR + 1
  const bxs: number[] = []
  let bx = 0
  branches.forEach((c) => { bxs.push(bx); bx += c.cols + HG })
  const relCenters = branches.map((c, i) => bxs[i] + c.entryCol)
  const entryCol = Math.round((relCenters[0] + relCenters[n - 1]) / 2)
  return {
    cols, rows, entryCol, exitCol: entryCol,
    paint(cv, x, y) {
      const splitRow = y
      const branchTop = y + 1
      const mergeRow = y + 1 + maxR
      const centers = relCenters.map((c) => x + c)
      const lo = Math.min(x + entryCol, ...centers)
      const hi = Math.max(x + entryCol, ...centers)
      cv.hline(lo, hi, splitRow)
      cv.hline(lo, hi, mergeRow)
      branches.forEach((c, i) => {
        const cx = x + bxs[i]
        const ce = cx + c.entryCol
        cv.vline(splitRow, branchTop, ce)
        c.paint(cv, cx, branchTop)
        cv.vline(branchTop + c.rows - 1, mergeRow, cx + c.exitCol)
      })
    },
  }
}

function oneOrMoreM(item: M, repLabel: string): M {
  const railGap = repLabel ? repLabel.length + 1 : 0
  const cols = item.cols + 1 + railGap
  const rows = item.rows + 2
  const railCol = item.cols   // one column right of the item block
  return {
    cols, rows, entryCol: item.entryCol, exitCol: item.exitCol,
    paint(cv, x, y) {
      const topRow = y
      const itemTop = y + 1
      const itemBot = y + item.rows           // last item row
      const botRow = y + item.rows + 1
      item.paint(cv, x, itemTop)
      cv.vline(topRow, itemTop, x + item.entryCol)
      cv.vline(itemBot, botRow, x + item.exitCol)
      const rc = x + railCol
      cv.hline(x + item.entryCol, rc, topRow)
      cv.hline(x + item.exitCol, rc, botRow)
      cv.vline(topRow, botRow, rc)
      if (repLabel) cv.text(y + Math.floor(rows / 2), rc + 1, repLabel)
    },
  }
}

function labelOf(node?: RailroadNode): string {
  if (!node) return ''
  if ('terminal' === node.kind || 'nonterminal' === node.kind) return node.text
  return '+'
}

function measure(node: RailroadNode, G: ReturnType<typeof glyphs>): M {
  switch (node.kind) {
    case 'terminal': return boxM(JSON.stringify(node.text), true, G)
    case 'nonterminal': return boxM(node.text, false, G)
    case 'comment': return commentM(node.text)
    case 'skip': return skipM()
    case 'seq': return seqM(node.items.map((n) => measure(n, G)))
    case 'choice': return choiceM(node.items.map((n) => measure(n, G)))
    case 'optional': return choiceM([measure(node.item, G), skipM()])
    case 'oneOrMore': return oneOrMoreM(measure(node.item, G), labelOf(node.rep))
    case 'zeroOrMore': return choiceM([oneOrMoreM(measure(node.item, G), labelOf(node.rep)), skipM()])
    case 'diagram': return seqM(node.items.map((n) => measure(n, G)))
    default:
      throw new RailroadError(
        'railroad: unknown node kind ' + JSON.stringify((node as any).kind), node)
  }
}


// Render a single node to an ASCII block.
export function renderNodeAscii(node: Item, opts: AsciiOptions = {}): string {
  const plain = !!opts.ascii
  const G = glyphs(plain)
  const cv = new Canvas()
  const m = measure(norm(node), G)
  // top + bottom rail caps, joined into the node's entry/exit cells
  cv.line(0, m.entryCol, D)
  m.paint(cv, 0, 1)
  cv.line(1, m.entryCol, U)
  cv.line(m.rows, m.exitCol, D)
  cv.line(m.rows + 1, m.exitCol, U)
  return cv.render(plain)
}


// Render a whole grammar: each rule as a titled vertical block.
export function modelToAscii(model: GrammarModel, opts: AsciiOptions = {}): string {
  const plain = !!opts.ascii
  const G = glyphs(plain)
  const names = orderRules(model)
  const blocks: string[] = []
  for (const name of names) {
    const cv = new Canvas()
    const m = measure(model.rules[name], G)
    cv.line(0, m.entryCol, D)
    m.paint(cv, 0, 1)
    cv.line(1, m.entryCol, U)
    cv.line(m.rows, m.exitCol, D)
    cv.line(m.rows + 1, m.exitCol, U)
    const diagram = cv.render(plain)
    blocks.push(name + ':\n' + diagram)
  }
  const keyBlock = (title: string, entries: { token: string; meaning: string }[]) => {
    const w = Math.max(...entries.map((e) => e.token.length))
    return title + ':\n' +
      entries.map((e) => '  ' + e.token.padEnd(w) + ' = ' + e.meaning).join('\n')
  }
  if (model.legend && model.legend.length) blocks.push(keyBlock('Tokens', model.legend))
  // Tokens the lexer silently skips (IGNORE set) — never appear in a rule.
  if (model.ignored && model.ignored.length) blocks.push(keyBlock('Ignored tokens', model.ignored))
  return blocks.join('\n\n')
}

function orderRules(model: GrammarModel): string[] {
  const names = Object.keys(model.rules)
  if (model.start && names.includes(model.start)) {
    return [model.start, ...names.filter((n) => n !== model.start)]
  }
  return names
}
