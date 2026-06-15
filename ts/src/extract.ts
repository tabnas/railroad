/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  extract.ts
 *  Build a railroad GrammarModel by introspecting a live @tabnas/parser
 *  instance. Reads the rule set (`tn.rule()`) and resolved config
 *  (`tn.internal().config`) and reverse-maps the alt-based rule machine
 *  into railroad constructs (sequence / choice / optional / repetition).
 *
 *  Mapping (see doc + tests against @tabnas/json):
 *   - open alt: first `s.length - b` token positions are consumed
 *     terminals; a `p:` push appends a nonterminal. `b == s.length`
 *     (pure peek) consumes nothing — render only the ref.
 *   - several open alts -> choice.
 *   - close alt `r: <self>` (+ guard token) -> repetition (OneOrMore with
 *     the guard token on the return path). `r: <other>` -> continuation.
 *     A close alt that consumes a token with no `b`/`r` is this rule's own
 *     closing terminal (append it). A `b:` backup close leaves the token
 *     for the parent -> drop. End-of-source / pure-pop -> drop.
 *   - synthetic helper rules (name has `$` or `_gen…`) are inlined.
 *   - normalization factors common prefix/suffix across choice branches
 *     and turns an empty branch into Optional.
 *
 *  Engine-agnostic about the introspection object: `tn` is read loosely
 *  so this never hard-depends on parser internals beyond the documented
 *  `rule()` / `internal().config` shape.
 */

import {
  RailroadNode,
  GrammarModel,
  Terminal,
  NonTerminal,
  Comment,
  Skip,
  Sequence,
  Choice,
  Optional,
  OneOrMore,
  ZeroOrMore,
  nodeEqual,
} from './model'


export type ExtractOptions = {
  // Apply prefix/suffix factoring + empty-branch->optional (default true).
  factor?: boolean
  // Override the entry rule (defaults to config.rule.start).
  start?: string
}

type Ctx = {
  cfg: any
  rsm: any
  tinName: Map<number, string>
  zz: number | undefined
  aa: number | undefined
  factor: boolean
}


export function extractGrammar(tn: any, opts: ExtractOptions = {}): GrammarModel {
  const internal = tn && 'function' === typeof tn.internal ? tn.internal() : {}
  const cfg = (internal && internal.config) ||
    (tn && 'function' === typeof tn.config ? tn.config() : {}) || {}
  const rsm = tn && 'function' === typeof tn.rule ? tn.rule() : {}

  const ctx: Ctx = {
    cfg,
    rsm,
    tinName: buildTinName(cfg),
    zz: cfg.t ? cfg.t['#ZZ'] : undefined,
    aa: cfg.t ? cfg.t['#AA'] : undefined,
    factor: opts.factor !== false,
  }

  // Entry rule; unwrap the synthetic `__start__` EOF wrapper.
  let start: string =
    opts.start || (cfg.rule && cfg.rule.start) || firstUserRule(rsm) || ''
  if ('__start__' === start && rsm['__start__']) {
    start = unwrapStart(rsm['__start__']) || start
  }

  const rules: { [name: string]: RailroadNode } = {}
  for (const name of Object.keys(rsm)) {
    if (!isUserRule(name)) continue
    rules[name] = ruleNode(name, ctx, new Set())
  }

  return { start, rules, meta: { engine: 'tabnas' } }
}


// ---- rule -> node --------------------------------------------------

function ruleNode(name: string, ctx: Ctx, visited: Set<string>): RailroadNode {
  const spec = ctx.rsm[name]
  if (!spec || !spec.def) return NonTerminal(name)
  if (visited.has(name) || visited.size > 32) return NonTerminal(name)
  const v2 = new Set(visited)
  v2.add(name)

  const open: any[] = spec.def.open || []
  const close: any[] = spec.def.close || []

  const branches = open
    .map((a) => openAltNode(a, ctx, v2))
    .filter((n): n is RailroadNode => !!n)

  let body: RailroadNode =
    0 === branches.length ? Skip()
      : 1 === branches.length ? branches[0]
        : Choice(...branches)

  body = applyCloseAlts(body, close, name, ctx, v2)
  if (ctx.factor) body = normalizeNode(body)
  return body
}


function openAltNode(alt: any, ctx: Ctx, visited: Set<string>): RailroadNode | null {
  const positions = altPositions(alt, ctx)
  const sN = 'number' === typeof alt.sN ? alt.sN : positions.length
  const consumed = Math.max(0, sN - numericBack(alt.b))

  const parts: RailroadNode[] = []
  for (let i = 0; i < consumed && i < positions.length; i++) {
    const pn = positionNode(positions[i], ctx)
    if (pn) parts.push(pn)
  }
  const ref = refNode(alt.p, ctx, visited)
  if (ref) parts.push(ref)

  if (0 === parts.length) return Skip()
  if (1 === parts.length) return parts[0]
  return Sequence(...parts)
}


function applyCloseAlts(
  body: RailroadNode, closeAlts: any[], ruleName: string, ctx: Ctx, visited: Set<string>,
): RailroadNode {
  let repeat = false
  let repSep: RailroadNode | undefined
  let closingTerm: RailroadNode | null = null
  let continuation: RailroadNode | null = null

  for (const alt of closeAlts) {
    const positions = altPositions(alt, ctx)
    const back = numericBack(alt.b)
    const sN = 'number' === typeof alt.sN ? alt.sN : positions.length
    const consumed = Math.max(0, sN - back)

    if ('string' === typeof alt.r) {
      if (alt.r === ruleName) {
        repeat = true
        if (consumed > 0) {
          const pn = positionNode(positions[0], ctx)
          if (pn) repSep = pn
        }
      } else {
        const c = refNode(alt.r, ctx, visited)
        if (c) continuation = c
      }
      continue
    }

    // No `r`: a backup close leaves the token for the parent -> drop.
    if (back > 0) continue

    // Consumes token(s) with no backup -> this rule's own closing terminal.
    const terms: RailroadNode[] = []
    for (let i = 0; i < consumed && i < positions.length; i++) {
      const pn = positionNode(positions[i], ctx)
      if (pn) terms.push(pn)
    }
    if (terms.length > 0) {
      closingTerm = 1 === terms.length ? terms[0] : Sequence(...terms)
    }
    // else: pure pop / end-of-source -> drop
  }

  let result = body
  if (repeat) result = OneOrMore(body, repSep)
  if (continuation) result = Sequence(...flattenSeq(result), continuation)
  if (closingTerm) result = Sequence(...flattenSeq(result), closingTerm)
  return result
}


// A push/replace ref -> inline if synthetic, else a nonterminal link.
function refNode(target: any, ctx: Ctx, visited: Set<string>): RailroadNode | null {
  if ('string' !== typeof target) {
    return target ? Comment('dynamic') : null
  }
  if (isSynthetic(target) && ctx.rsm[target]) {
    return ruleNode(target, ctx, visited)
  }
  return NonTerminal(target)
}


// ---- token / position resolution -----------------------------------

type Position = { tins: number[]; raw?: string }

function altPositions(alt: any, ctx: Ctx): Position[] {
  let sArr: any = alt.s
  if ('string' === typeof sArr) sArr = sArr.trim().split(/\s+/).filter(Boolean)
  if (!Array.isArray(sArr)) sArr = []
  const t: any[] = Array.isArray(alt.t) ? alt.t : []
  const n = Math.max(t.length, sArr.length)
  const out: Position[] = []
  for (let i = 0; i < n; i++) {
    const raw = 'string' === typeof sArr[i] ? sArr[i] : undefined
    let tins: number[] = Array.isArray(t[i])
      ? t[i].filter((x: any) => 'number' === typeof x)
      : resolveRaw(raw, ctx)
    out.push({ tins, raw })
  }
  return out
}

// One token position -> a node (or null if it's pure control tokens).
function positionNode(pos: Position, ctx: Ctx): RailroadNode | null {
  // Prefer the raw `#`-prefixed set name (e.g. #KEY, #VAL) for readability.
  if ('string' === typeof pos.raw) {
    const bare = stripHash(pos.raw)
    if (/^#[A-Za-z]/.test(pos.raw) && ctx.cfg.tokenSet && ctx.cfg.tokenSet[bare]) {
      return Terminal(bare)
    }
  }
  const useful = pos.tins.filter((t) => !isControl(t, ctx))
  if (0 === useful.length) return null
  if (1 === useful.length) return Terminal(tokenLabel(useful[0], ctx))
  const setName = matchTokenSet(useful, ctx)
  if (setName) return Terminal(setName)
  return Choice(...useful.map((t) => Terminal(tokenLabel(t, ctx))))
}

function tokenLabel(tin: number, ctx: Ctx): string {
  const fixed = ctx.cfg.fixed && ctx.cfg.fixed.ref && ctx.cfg.fixed.ref[tin]
  if ('string' === typeof fixed) return fixed
  const nm = ctx.tinName.get(tin)
  if (nm) return stripHash(nm)
  const m = ctx.cfg.match && ctx.cfg.match.token && ctx.cfg.match.token[tin]
  if (m instanceof RegExp) return prettySource(m)
  return '#' + tin
}

function matchTokenSet(tins: number[], ctx: Ctx): string | null {
  const sets = ctx.cfg.tokenSet
  if (!sets) return null
  const a = new Set(tins)
  for (const name of Object.keys(sets)) {
    const members = sets[name]
    if (!Array.isArray(members) || members.length !== a.size) continue
    if (members.every((m: number) => a.has(m))) return name
  }
  return null
}

function resolveRaw(raw: string | undefined, ctx: Ctx): number[] {
  if ('string' !== typeof raw) return []
  const bare = stripHash(raw)
  if (ctx.cfg.tokenSet && Array.isArray(ctx.cfg.tokenSet[bare])) {
    return ctx.cfg.tokenSet[bare].filter((x: any) => 'number' === typeof x)
  }
  const t = ctx.cfg.t && ctx.cfg.t[raw]
  return 'number' === typeof t ? [t] : []
}


// ---- normalization passes ------------------------------------------

function normalizeNode(node: RailroadNode): RailroadNode {
  switch (node.kind) {
    case 'seq':
      return seqOf(node.items.map(normalizeNode))
    case 'choice':
      return factorChoice(node.items.map(normalizeNode))
    case 'optional':
      return Optional(normalizeNode(node.item))
    case 'oneOrMore':
      return OneOrMore(
        normalizeNode(node.item), node.rep ? normalizeNode(node.rep) : undefined)
    case 'zeroOrMore':
      return ZeroOrMore(
        normalizeNode(node.item), node.rep ? normalizeNode(node.rep) : undefined)
    case 'diagram':
      return { kind: 'diagram', items: node.items.map(normalizeNode) }
    default:
      return node
  }
}

function factorChoice(rawBranches: RailroadNode[]): RailroadNode {
  // Dedup identical branches.
  let branches: RailroadNode[] = []
  for (const b of rawBranches) if (!branches.some((u) => nodeEqual(u, b))) branches.push(b)
  if (1 === branches.length) return branches[0]

  // Separate empty (skip) branches.
  let hasEmpty = false
  const nonEmpty = branches.filter((b) => {
    if ('skip' === b.kind) { hasEmpty = true; return false }
    return true
  })
  if (0 === nonEmpty.length) return Skip()

  const seqs = nonEmpty.map(asSeqItems)

  // Common prefix.
  const prefix: RailroadNode[] = []
  prefixLoop: for (let i = 0; ; i++) {
    const first = seqs[0][i]
    if (undefined === first) break
    for (const s of seqs) if (undefined === s[i] || !nodeEqual(s[i], first)) break prefixLoop
    prefix.push(first)
  }

  // Common suffix (not overlapping the prefix).
  const suffix: RailroadNode[] = []
  const minTail = Math.min(...seqs.map((s) => s.length - prefix.length))
  suffixLoop: for (let k = 1; k <= minTail; k++) {
    const first = seqs[0][seqs[0].length - k]
    if (undefined === first) break
    for (const s of seqs) {
      const el = s[s.length - k]
      if (undefined === el || !nodeEqual(el, first)) break suffixLoop
    }
    suffix.unshift(first)
  }

  // Remainders between prefix and suffix.
  let remEmpty = false
  let remNodes: RailroadNode[] = []
  for (const s of seqs) {
    const rem = seqOf(s.slice(prefix.length, s.length - suffix.length))
    if ('skip' === rem.kind) { remEmpty = true; continue }
    if (!remNodes.some((u) => nodeEqual(u, rem))) remNodes.push(rem)
  }

  let core: RailroadNode
  if (0 === remNodes.length) core = Skip()
  else if (1 === remNodes.length) core = remNodes[0]
  else core = { kind: 'choice', items: remNodes }

  if ((hasEmpty || remEmpty) && 'skip' !== core.kind) core = Optional(core)

  return seqOf([...prefix, ...('skip' === core.kind ? [] : [core]), ...suffix])
}

function asSeqItems(node: RailroadNode): RailroadNode[] {
  if ('seq' === node.kind) return node.items.slice()
  if ('skip' === node.kind) return []
  return [node]
}

function flattenSeq(node: RailroadNode): RailroadNode[] {
  return 'seq' === node.kind ? node.items.slice() : ['skip' === node.kind ? null : node].filter(Boolean) as RailroadNode[]
}

function seqOf(items: RailroadNode[]): RailroadNode {
  const flat: RailroadNode[] = []
  for (const it of items) {
    if ('seq' === it.kind) flat.push(...it.items)
    else if ('skip' === it.kind) continue
    else flat.push(it)
  }
  if (0 === flat.length) return Skip()
  if (1 === flat.length) return flat[0]
  return { kind: 'seq', items: flat }
}


// ---- small helpers -------------------------------------------------

function buildTinName(cfg: any): Map<number, string> {
  const map = new Map<number, string>()
  const t = cfg && cfg.t
  if (!t) return map
  const put = (tin: number, name: string) => {
    const cur = map.get(tin)
    // Prefer a `#`-prefixed name over the stripped alias.
    if (undefined === cur || (name.startsWith('#') && !cur.startsWith('#'))) {
      map.set(tin, name)
    }
  }
  for (const [k, v] of Object.entries(t)) {
    if ('number' === typeof v) put(v as number, k)            // name -> tin
    else if ('string' === typeof v && /^\d+$/.test(k)) put(Number(k), v) // tin -> name
  }
  return map
}

function isControl(tin: number, ctx: Ctx): boolean {
  return tin === ctx.zz || tin === ctx.aa
}

function numericBack(b: any): number {
  if ('number' === typeof b) return b
  return b ? 1 : 0   // function/object backup -> assume single-token peek
}

function isSynthetic(name: string): boolean {
  return name.includes('$') || /^_gen\d/.test(name)
}

function isUserRule(name: string): boolean {
  return '__start__' !== name && !isSynthetic(name)
}

function firstUserRule(rsm: any): string | undefined {
  return Object.keys(rsm).find(isUserRule)
}

function unwrapStart(spec: any): string | undefined {
  const open = spec && spec.def && spec.def.open
  if (Array.isArray(open) && open[0] && 'string' === typeof open[0].p) return open[0].p
  return undefined
}

function stripHash(s: string): string {
  return s.replace(/^#/, '')
}

function prettySource(re: RegExp): string {
  return re.source.replace(/^\^/, '')
}
