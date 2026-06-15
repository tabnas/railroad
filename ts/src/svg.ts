/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  svg.ts
 *  Vertical-flow SVG renderer for railroad diagrams. Flow runs top->bottom:
 *  sequences stack vertically, choice branches fan out side-by-side, and
 *  optional / repetition rails run parallel on the side. This biases the
 *  output tall-and-narrow, which suits laptop browsers and phones.
 *
 *  Each node measures to a `VLayout { width, height, entryX, exitX, draw }`.
 *  The rail enters the top edge at `entryX` and leaves the bottom edge at
 *  `exitX` (both equal — all nodes are horizontally symmetric). `draw(x,y)`
 *  emits SVG with the node's bounding box at top-left `(x,y)`.
 *
 *  `modelToSvg` stacks one titled, anchored sub-diagram per rule and turns
 *  nonterminal boxes into `<a href="#rule">` links.
 */

import { RailroadNode, GrammarModel, Item, norm, RailroadError } from './model'


// ---- geometry constants --------------------------------------------
const CHARW = 8
const PADX = 10
const BOXH = 26
const MINW = 30
const VGAP = 18    // vertical gap between stacked items / split-merge stubs
const HGAP = 26    // horizontal gap between choice branches
const AR = 10      // loop rail inset
const PAD = 16     // outer padding
const TITLE_H = 26 // height reserved for a rule title
const TRACK_GAP = 34
const LEAD = 14    // rail lead between a cap dot and the content


export type SvgOptions = {
  // (name) => href for nonterminal boxes (whole-grammar linking).
  linkFor?: (name: string) => string | undefined
}

type VLayout = {
  width: number
  height: number
  entryX: number
  exitX: number
  isSkip?: boolean   // a bypass branch — gets a tighter gap in a choice
  draw: (x: number, y: number) => string
}


// ---- primitives ----------------------------------------------------
function esc(s: string): string {
  return s.replace(/[&<>"]/g, (c) =>
    ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c] as string))
}
const path = (d: string, cls = 'rr-line') => `<path class="${cls}" d="${d}"/>`
const vline = (x: number, y1: number, y2: number) => y1 === y2 ? '' : path(`M${x} ${y1}V${y2}`)
const hline = (x1: number, x2: number, y: number) => x1 === x2 ? '' : path(`M${x1} ${y}H${x2}`)
const cap = (x: number, y: number) => `<circle class="rr-cap" cx="${x}" cy="${y}" r="3"/>`


// ---- node layouts --------------------------------------------------
function boxLayout(text: string, cls: string, terminal: boolean, href?: string): VLayout {
  const w = Math.max(text.length * CHARW + 2 * PADX, MINW)
  const h = BOXH
  return {
    width: w, height: h, entryX: w / 2, exitX: w / 2,
    draw(x, y) {
      const r = terminal ? h / 2 : 4
      const inner =
        `<rect class="${cls}" x="${x}" y="${y}" width="${w}" height="${h}" rx="${r}" ry="${r}"/>` +
        `<text class="rr-label" x="${x + w / 2}" y="${y + h / 2}">${esc(text)}</text>`
      return href ? `<a href="${esc(href)}">${inner}</a>` : inner
    },
  }
}

function commentLayout(text: string): VLayout {
  const w = Math.max(text.length * CHARW + 2 * PADX, MINW)
  return {
    width: w, height: BOXH, entryX: w / 2, exitX: w / 2,
    draw(x, y) {
      return vline(x + w / 2, y, y + BOXH) +
        `<text class="rr-comment" x="${x + w / 2}" y="${y + BOXH / 2}">${esc(text)}</text>`
    },
  }
}

function skipLayout(): VLayout {
  const w = 16, h = 12
  return {
    width: w, height: h, entryX: w / 2, exitX: w / 2, isSkip: true,
    draw(x, y) { return vline(x + w / 2, y, y + h) },
  }
}

function seqLayout(children: VLayout[]): VLayout {
  if (0 === children.length) return skipLayout()
  if (1 === children.length) return children[0]
  const railX = Math.max(...children.map((c) => c.entryX))
  const offs = children.map((c) => railX - c.entryX)
  const width = children.reduce((m, c, i) => Math.max(m, offs[i] + c.width), 0)
  const height = children.reduce((a, c) => a + c.height, 0) + VGAP * (children.length - 1)
  return {
    width, height, entryX: railX, exitX: railX,
    draw(x, y) {
      let out = ''
      let cy = y
      children.forEach((c, i) => {
        if (i > 0) { out += vline(x + railX, cy - VGAP, cy) }
        out += c.draw(x + offs[i], cy)
        cy += c.height + VGAP
      })
      return out
    },
  }
}

function choiceLayout(branches: VLayout[]): VLayout {
  if (1 === branches.length) return branches[0]
  const n = branches.length
  const maxBH = Math.max(...branches.map((c) => c.height))
  // entry stub + fan-in + branches + fan-out + exit stub
  const height = 4 * VGAP + maxBH
  // branch x offsets within the box, and the entry on the fan midpoint.
  // A bypass (skip) branch hugs its neighbour with a tighter gap.
  const bxs: number[] = []
  let bx = 0
  branches.forEach((c, i) => {
    bxs.push(bx)
    const next = branches[i + 1]
    const gap = next && (c.isSkip || next.isSkip) ? AR : HGAP
    bx += c.width + gap
  })
  const width = bxs[n - 1] + branches[n - 1].width
  const relCenters = branches.map((c, i) => bxs[i] + c.entryX)
  const entryX = (relCenters[0] + relCenters[n - 1]) / 2
  return {
    width, height, entryX, exitX: entryX,
    draw(x, y) {
      const splitY = y + VGAP
      const branchY = splitY + VGAP
      const mergeY = branchY + maxBH + VGAP
      const centers = relCenters.map((c) => x + c)
      const lo = Math.min(x + entryX, ...centers)
      const hi = Math.max(x + entryX, ...centers)
      let out = ''
      out += vline(x + entryX, y, splitY)            // entry stub
      out += hline(lo, hi, splitY)                   // split rail
      out += hline(lo, hi, mergeY)                   // merge rail
      out += vline(x + entryX, mergeY, y + height)   // exit stub
      branches.forEach((c, i) => {
        const cx = x + bxs[i]
        const bex = cx + c.entryX
        out += vline(bex, splitY, branchY)            // fan-in drop
        out += c.draw(cx, branchY)
        out += vline(bex, branchY + c.height, mergeY) // fan-out drop
      })
      return out
    },
  }
}

function oneOrMoreLayout(item: VLayout, rep?: VLayout): VLayout {
  const repW = rep ? rep.width : 0
  const railX0 = item.width + AR + repW / 2          // x of the return rail (box-relative)
  const width = item.width + 2 * AR + repW
  const topGap = VGAP, botGap = VGAP
  const height = topGap + item.height + botGap
  const entryX = item.entryX
  return {
    width, height, entryX, exitX: item.exitX,
    draw(x, y) {
      const mainX = x + entryX
      const itemY = y + topGap
      const railX = x + railX0
      const loopTopY = y + topGap / 2
      const loopBottomY = y + height - botGap / 2
      let out = ''
      out += vline(mainX, y, itemY)
      out += item.draw(x, itemY)
      out += vline(x + item.exitX, itemY + item.height, y + height)
      out += hline(mainX, railX, loopBottomY)
      out += hline(mainX, railX, loopTopY)
      if (rep) {
        const repH = rep.height
        const midY = (loopTopY + loopBottomY) / 2
        const rTop = midY - repH / 2
        const rBot = midY + repH / 2
        out += vline(railX, rBot, loopBottomY)
        out += vline(railX, loopTopY, rTop)
        out += rep.draw(railX - rep.entryX, rTop)
      } else {
        out += vline(railX, loopTopY, loopBottomY)
      }
      return out
    },
  }
}

function layoutNode(node: RailroadNode, opts: SvgOptions): VLayout {
  switch (node.kind) {
    case 'terminal':
      return boxLayout(node.text, 'rr-term', true)
    case 'nonterminal':
      return boxLayout(node.text, 'rr-nonterm', false,
        opts.linkFor ? opts.linkFor(node.text) : undefined)
    case 'comment':
      return commentLayout(node.text)
    case 'skip':
      return skipLayout()
    case 'seq':
      return seqLayout(node.items.map((n) => layoutNode(n, opts)))
    case 'choice':
      return choiceLayout(node.items.map((n) => layoutNode(n, opts)))
    case 'optional':
      return choiceLayout([layoutNode(node.item, opts), skipLayout()])
    case 'oneOrMore':
      return oneOrMoreLayout(
        layoutNode(node.item, opts), node.rep ? layoutNode(node.rep, opts) : undefined)
    case 'zeroOrMore':
      return choiceLayout([
        oneOrMoreLayout(
          layoutNode(node.item, opts), node.rep ? layoutNode(node.rep, opts) : undefined),
        skipLayout(),
      ])
    case 'diagram':
      return seqLayout(node.items.map((n) => layoutNode(n, opts)))
    default:
      throw new RailroadError(
        'railroad: unknown node kind ' + JSON.stringify((node as any).kind), node)
  }
}


// ---- document assembly ---------------------------------------------
const STYLE =
  'svg.railroad{background:#fff;font-family:monospace;font-size:13px}' +
  '.rr-line{stroke:#334;stroke-width:2;fill:none}' +
  '.rr-cap{fill:#334}' +
  '.rr-term{fill:#e8f0ff;stroke:#334;stroke-width:2}' +
  '.rr-nonterm{fill:#fff7e8;stroke:#334;stroke-width:2}' +
  '.rr-label{fill:#111;text-anchor:middle;dominant-baseline:middle}' +
  '.rr-comment{fill:#666;font-style:italic;text-anchor:middle;dominant-baseline:middle}' +
  '.rr-title{fill:#113;font-weight:bold;font-size:15px}' +
  'a:hover .rr-nonterm{fill:#ffe6b3;cursor:pointer}'

function svgDoc(body: string, w: number, h: number): string {
  const W = Math.ceil(w), H = Math.ceil(h)
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" class="railroad" ` +
    `width="${W}" height="${H}" viewBox="0 0 ${W} ${H}">` +
    `<style>${STYLE}</style><g>${body}</g></svg>`
  )
}


// Draw a layout with entry/exit cap dots, separated from the content by a
// short rail lead so the dots never sit on top of a box.
function withCaps(L: VLayout, x: number, top: number): string {
  const ct = top + LEAD
  const cb = ct + L.height
  const ex = x + L.entryX, xx = x + L.exitX
  return (
    cap(ex, top) + vline(ex, top, ct) +
    L.draw(x, ct) +
    vline(xx, cb, cb + LEAD) + cap(xx, cb + LEAD)
  )
}

// Render a single node (wrapped in its own SVG document).
export function renderNodeSvg(node: Item, opts: SvgOptions = {}): string {
  const L = layoutNode(norm(node), opts)
  return svgDoc(withCaps(L, PAD, PAD), L.width + 2 * PAD, L.height + 2 * LEAD + 2 * PAD)
}


// Render a whole grammar: a vertical stack of titled, anchored rule
// tracks; nonterminal boxes link to the referenced rule's track.
export function modelToSvg(model: GrammarModel, _opts: SvgOptions = {}): string {
  const ruleNames = new Set(Object.keys(model.rules))
  const linkFor = (name: string) => (ruleNames.has(name) ? '#' + name : undefined)
  const opts: SvgOptions = { linkFor }

  // Measure each rule track (title + capped diagram).
  const order = orderRules(model)
  type Track = { name: string; L: VLayout; h: number }
  const tracks: Track[] = order.map((name) => {
    const L = layoutNode(model.rules[name], opts)
    return { name, L, h: TITLE_H + L.height + 2 * LEAD }
  })

  // Lay the tracks out in two newspaper-style columns (filled top to
  // bottom, balanced by height), using the available horizontal space.
  const twoCol = tracks.length > 1
  const total = tracks.reduce((a, t) => a + t.h + TRACK_GAP, 0)
  const cols: Track[][] = [[], []]
  let acc = 0, ci = 0
  for (const t of tracks) {
    cols[ci].push(t)
    acc += t.h + TRACK_GAP
    if (twoCol && 0 === ci && acc >= total / 2) ci = 1
  }

  const COLGAP = 48
  let x = PAD
  let pageH = 0
  let body = ''
  for (const col of cols) {
    if (0 === col.length) continue
    let y = PAD
    let colW = 0
    for (const t of col) {
      const dy = y + TITLE_H
      body += `<g id="${esc(t.name)}">`
      body += `<text class="rr-title" x="${x}" y="${y + 15}">${esc(t.name)}</text>`
      body += withCaps(t.L, x, dy)
      body += `</g>`
      colW = Math.max(colW, t.L.width)
      y = dy + t.L.height + 2 * LEAD + TRACK_GAP
    }
    pageH = Math.max(pageH, y - TRACK_GAP)
    x += colW + COLGAP
  }
  return svgDoc(body, x - COLGAP + PAD, pageH + PAD)
}

function orderRules(model: GrammarModel): string[] {
  const names = Object.keys(model.rules)
  if (model.start && names.includes(model.start)) {
    return [model.start, ...names.filter((n) => n !== model.start)]
  }
  return names
}
