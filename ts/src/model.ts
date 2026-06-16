/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  model.ts
 *  Railroad diagram model — a small, engine-agnostic tree of nodes
 *  (terminal, nonterminal, sequence, choice, optional, repetition) plus
 *  the `GrammarModel` envelope (one node per grammar rule). The model is
 *  pure JSON-serializable data: it is the interchange format that the
 *  SVG and ASCII renderers consume, and that `extract.ts` produces from a
 *  live tabnas instance.
 */


// ---- diagram model -------------------------------------------------

export type RailroadNode =
  | { kind: 'terminal'; text: string }
  | { kind: 'nonterminal'; text: string }
  | { kind: 'comment'; text: string }
  | { kind: 'skip' }
  | { kind: 'seq'; items: RailroadNode[] }
  | { kind: 'choice'; items: RailroadNode[] }
  | { kind: 'optional'; item: RailroadNode }
  | { kind: 'oneOrMore'; item: RailroadNode; rep?: RailroadNode }
  | { kind: 'zeroOrMore'; item: RailroadNode; rep?: RailroadNode }
  | { kind: 'diagram'; items: RailroadNode[] }


// One whole grammar: an ordered rule map plus the entry rule. This is the
// declarative artifact emitted as `grammar.railroad.json`.
export type GrammarModel = {
  start: string
  rules: { [name: string]: RailroadNode }
  // Key for the named tokens (non-literal labels) that appear in the diagram.
  legend?: { token: string; meaning: string }[]
  // Tokens the lexer silently skips (the IGNORE set): whitespace, comments,
  // etc. They never appear in a rule, so they are reported separately.
  ignored?: { token: string; meaning: string }[]
  meta?: { engine: string; [k: string]: any }
}


// Raised on a malformed diagram model (empty choice, unknown kind, ...).
export class RailroadError extends Error {
  node?: unknown
  constructor(message: string, node?: unknown) {
    super(message)
    this.name = 'RailroadError'
    this.node = node
  }
}


// A bare string argument is taken to be a terminal — the common case
// when hand-building diagrams: `seq('(', expr, ')')`.
export type Item = RailroadNode | string

export function norm(item: Item): RailroadNode {
  if ('string' === typeof item) return Terminal(item)
  if (!item || 'string' !== typeof (item as any).kind) {
    throw new RailroadError('railroad: invalid diagram node', item)
  }
  return item
}
function normAll(items: Item[]): RailroadNode[] {
  return items.map(norm)
}


export function Terminal(text: string): RailroadNode {
  return { kind: 'terminal', text: String(text) }
}
export function NonTerminal(text: string): RailroadNode {
  return { kind: 'nonterminal', text: String(text) }
}
export function Comment(text: string): RailroadNode {
  return { kind: 'comment', text: String(text) }
}
export function Skip(): RailroadNode {
  return { kind: 'skip' }
}
export function Sequence(...items: Item[]): RailroadNode {
  return { kind: 'seq', items: normAll(items) }
}
export function Choice(...items: Item[]): RailroadNode {
  if (items.length < 1) {
    throw new RailroadError('railroad: choice needs at least one branch')
  }
  return { kind: 'choice', items: normAll(items) }
}
export function Optional(item: Item): RailroadNode {
  return { kind: 'optional', item: norm(item) }
}
export function OneOrMore(item: Item, rep?: Item): RailroadNode {
  return { kind: 'oneOrMore', item: norm(item), rep: rep == null ? undefined : norm(rep) }
}
export function ZeroOrMore(item: Item, rep?: Item): RailroadNode {
  return { kind: 'zeroOrMore', item: norm(item), rep: rep == null ? undefined : norm(rep) }
}
export function Diagram(...items: Item[]): RailroadNode {
  return { kind: 'diagram', items: normAll(items) }
}


// ---- text emitter --------------------------------------------------

// Compact EBNF-ish rendering: terminals quoted, choice in (a | b),
// optional in [x], zero-or-more in {x}, one-or-more as x+.
export function toText(node: Item): string {
  const n = norm(node)
  switch (n.kind) {
    case 'terminal':
      return JSON.stringify(n.text)
    case 'nonterminal':
      return n.text
    case 'comment':
      return '/* ' + n.text + ' */'
    case 'skip':
      return ''
    case 'seq':
      return n.items.map(toText).filter((s) => '' !== s).join(' ')
    case 'choice':
      return '(' + n.items.map(toText).join(' | ') + ')'
    case 'optional':
      return '[' + toText(n.item) + ']'
    case 'oneOrMore':
      return toText(n.item) + '+' + (n.rep ? ' /* ' + toText(n.rep) + ' */' : '')
    case 'zeroOrMore':
      return '{' + toText(n.item) + '}' + (n.rep ? ' /* ' + toText(n.rep) + ' */' : '')
    case 'diagram':
      return n.items.map(toText).filter((s) => '' !== s).join(' ')
    default:
      throw new RailroadError(
        'railroad: unknown node kind ' + JSON.stringify((n as any).kind), n)
  }
}


// ---- structural helpers (used by extract normalization + renderers) --

// Deep structural equality on nodes — used by choice prefix/suffix
// factoring to detect shared leading/trailing elements.
export function nodeEqual(a: RailroadNode, b: RailroadNode): boolean {
  if (a.kind !== b.kind) return false
  switch (a.kind) {
    case 'terminal':
    case 'nonterminal':
    case 'comment':
      return a.text === (b as any).text
    case 'skip':
      return true
    case 'seq':
    case 'choice':
    case 'diagram': {
      const bi = (b as any).items as RailroadNode[]
      if (a.items.length !== bi.length) return false
      return a.items.every((it, i) => nodeEqual(it, bi[i]))
    }
    case 'optional':
      return nodeEqual(a.item, (b as any).item)
    case 'oneOrMore':
    case 'zeroOrMore': {
      const bb = b as any
      const repEq = (!a.rep && !bb.rep) || (!!a.rep && !!bb.rep && nodeEqual(a.rep, bb.rep))
      return repEq && nodeEqual(a.item, bb.item)
    }
  }
}
