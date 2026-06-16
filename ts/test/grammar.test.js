/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */
'use strict'

// Grammar-driven extraction + rendering, validated against @tabnas/json.
// (ini/yaml are deferred — they target the older @tabnas/jsonic engine,
// not @tabnas/parser, so they are out of scope for this renderer.)

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Os = require('node:os')
const Path = require('node:path')

const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const {
  railroad: railroadPlugin,
  extractGrammar,
  modelToSvg,
  modelToAscii,
} = require('..')
const RailroadCli = require('../dist/bin/tabnas-railroad-cli')


function build() {
  return new Tabnas({ plugins: [json, railroadPlugin] })
}


describe('grammar extraction (json)', () => {

  it('extracts the rule set and start rule', () => {
    const model = build().railroad.toJson()
    assert.equal(model.start, 'val')
    assert.deepEqual(Object.keys(model.rules).sort(), ['elem', 'list', 'map', 'pair', 'val'])
    assert.equal(model.meta.engine, 'tabnas')
  })

  it('val is a choice of map | list | VAL', () => {
    const { val } = build().railroad.toJson().rules
    assert.equal(val.kind, 'choice')
    const kinds = val.items.map((i) => i.kind + ':' + (i.text || ''))
    assert.ok(kinds.includes('nonterminal:map'))
    assert.ok(kinds.includes('nonterminal:list'))
    assert.ok(val.items.some((i) => i.kind === 'terminal' && i.text === 'VAL'))
  })

  it('map is "{" [pair] "}"', () => {
    const { map } = build().railroad.toJson().rules
    assert.equal(map.kind, 'seq')
    assert.deepEqual(map.items[0], { kind: 'terminal', text: '{' })
    assert.equal(map.items[1].kind, 'optional')
    assert.deepEqual(map.items[1].item, { kind: 'nonterminal', text: 'pair' })
    assert.deepEqual(map.items[2], { kind: 'terminal', text: '}' })
  })

  it('list is "[" [elem] "]"', () => {
    const { list } = build().railroad.toJson().rules
    assert.equal(list.kind, 'seq')
    assert.deepEqual(list.items[0], { kind: 'terminal', text: '[' })
    assert.equal(list.items[1].kind, 'optional')
    assert.deepEqual(list.items[1].item, { kind: 'nonterminal', text: 'elem' })
    assert.deepEqual(list.items[2], { kind: 'terminal', text: ']' })
  })

  it('pair is (KEY ":" val) repeated, separated by ","', () => {
    const { pair } = build().railroad.toJson().rules
    assert.equal(pair.kind, 'oneOrMore')
    assert.deepEqual(pair.rep, { kind: 'terminal', text: ',' })
    assert.equal(pair.item.kind, 'seq')
    assert.deepEqual(pair.item.items[0], { kind: 'terminal', text: 'KEY' })
    assert.deepEqual(pair.item.items[1], { kind: 'terminal', text: ':' })
    assert.deepEqual(pair.item.items[2], { kind: 'nonterminal', text: 'val' })
  })

  it('elem is val repeated, separated by ","', () => {
    const { elem } = build().railroad.toJson().rules
    assert.equal(elem.kind, 'oneOrMore')
    assert.deepEqual(elem.item, { kind: 'nonterminal', text: 'val' })
    assert.deepEqual(elem.rep, { kind: 'terminal', text: ',' })
  })

  it('extractGrammar() bare export matches tn.railroad.toJson()', () => {
    const tn = build()
    assert.deepEqual(extractGrammar(tn), tn.railroad.toJson())
  })

})


describe('whole-grammar rendering (json)', () => {

  it('SVG is well-formed, anchored, and links nonterminals', () => {
    const tn = build()
    const svg = tn.railroad.toSvg()
    assert.match(svg, /^<svg /)
    assert.match(svg, /<\/svg>$/)
    const w = Number(svg.match(/width="(\d+)"/)[1])
    const h = Number(svg.match(/height="(\d+)"/)[1])
    assert.ok(w > 0 && h > 0)
    assert.ok(h > w, 'expected a vertically-biased (taller-than-wide) diagram')
    for (const r of ['val', 'map', 'list', 'pair', 'elem']) {
      assert.ok(svg.includes(`id="${r}"`), `missing track anchor id="${r}"`)
    }
    assert.ok(svg.includes('<a href="#map"'), 'nonterminal map should link')
    assert.ok(svg.includes('<a href="#val"'), 'nonterminal val should link')
  })

  it('ASCII contains every rule name', () => {
    const ascii = build().railroad.toAscii()
    for (const r of ['val', 'map', 'list', 'pair', 'elem']) {
      assert.ok(ascii.includes(r + ':'), `missing rule heading ${r}:`)
    }
  })

  it('ASCII plain mode is pure ASCII', () => {
    const ascii = build().railroad.toAscii({ ascii: true })
    // eslint-disable-next-line no-control-regex
    assert.ok(/^[\x00-\x7F]*$/.test(ascii), 'expected pure ASCII output')
  })

  it('model round-trips through JSON to the same SVG', () => {
    const model = build().railroad.toJson()
    const clone = JSON.parse(JSON.stringify(model))
    assert.equal(modelToSvg(clone), modelToSvg(model))
    assert.equal(modelToAscii(clone), modelToAscii(model))
  })

  it('emits a token key/legend, rendered in SVG and ASCII', () => {
    const model = build().railroad.toJson()
    assert.ok(Array.isArray(model.legend) && model.legend.length > 0,
      'model should carry a token legend')
    const meaning = Object.fromEntries(model.legend.map((e) => [e.token, e.meaning]))
    // json renders { / : / } as literals, but the KEY/VAL token sets show as
    // names and therefore need a key entry.
    assert.ok('KEY' in meaning, 'legend should explain KEY')
    assert.ok('VAL' in meaning, 'legend should explain VAL')
    assert.match(meaning.VAL, /value/)
    // the key is rendered into both outputs.
    assert.ok(modelToAscii(model).includes('Tokens:'), 'ASCII should include a Tokens key')
    assert.ok(modelToSvg(model).includes('>Tokens<'), 'SVG should include a Tokens key')
  })

  it('reports the IGNORE set (ignored tokens), rendered in SVG and ASCII', () => {
    const model = build().railroad.toJson()
    assert.ok(Array.isArray(model.ignored) && model.ignored.length > 0,
      'model should carry the ignored-token set')
    const tokens = model.ignored.map((e) => e.token)
    // json ignores whitespace, newlines and comments — none appear in a rule.
    assert.ok(tokens.includes('SP'), 'SP should be reported as ignored')
    assert.ok(tokens.includes('LN'), 'LN should be reported as ignored')
    assert.ok(model.ignored.every((e) => 'string' === typeof e.meaning && e.meaning),
      'every ignored token should carry a meaning')
    assert.ok(modelToAscii(model).includes('Ignored tokens:'),
      'ASCII should include an Ignored tokens key')
    assert.ok(modelToSvg(model).includes('>Ignored tokens<'),
      'SVG should include an Ignored tokens key')
  })

})


describe('cli', () => {

  it('grammar mode writes the three artifacts', async () => {
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'rr-'))
    const cn = makeConsole()
    await RailroadCli.run([0, 0, '--grammar', '@tabnas/json', '-o', dir], cn)
    assert.equal(cn.d.err.length, 0, cn.d.err.join('\n'))
    const json = Fs.readFileSync(Path.join(dir, 'grammar.railroad.json'), 'utf8')
    const svg = Fs.readFileSync(Path.join(dir, 'grammar.svg'), 'utf8')
    const txt = Fs.readFileSync(Path.join(dir, 'grammar.txt'), 'utf8')
    assert.equal(JSON.parse(json).start, 'val')
    assert.match(svg, /^<svg /)
    assert.match(svg, /<\/svg>$/)
    assert.ok(txt.includes('val:'))
  })

  it('render mode renders a saved model from -f', async () => {
    const dir = Fs.mkdtempSync(Path.join(Os.tmpdir(), 'rr-'))
    const model = build().railroad.toJson()
    const file = Path.join(dir, 'm.json')
    Fs.writeFileSync(file, JSON.stringify(model))
    const cn = makeConsole()
    await RailroadCli.run([0, 0, '-f', file, '--text'], cn)
    assert.match(cn.d.log[0][0], /^val = /m)
  })

  it('prints help with -h', async () => {
    const cn = makeConsole()
    await RailroadCli.run([0, 0, '-h'], cn)
    assert.match(cn.d.log[0][0], /Usage:/)
  })

})


function makeConsole() {
  const d = { log: [], err: [] }
  return {
    d,
    log: (...rest) => d.log.push(rest),
    error: (...rest) => d.err.push(rest),
  }
}
