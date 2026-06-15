/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')

const { Tabnas } = require('@tabnas/parser')
const {
  railroad: railroadPlugin,
  renderNodeSvg,
  renderNodeAscii,
  toText,
  Diagram,
  Sequence,
  Choice,
  Optional,
  OneOrMore,
  ZeroOrMore,
  Terminal,
  NonTerminal,
  RailroadError,
} = require('..')


describe('railroad (node-level)', () => {

  // The plugin decorates the instance with the grammar-driven API.
  describe('plugin load', () => {

    it('loads via the constructor plugins array', () => {
      const tn = new Tabnas({ plugins: [railroadPlugin] })
      assert.equal(typeof tn.railroad, 'function')
      for (const k of ['toJson', 'toSvg', 'toAscii', 'extract', 'renderNode', 'renderNodeAscii']) {
        assert.equal(typeof tn.railroad[k], 'function', `missing tn.railroad.${k}`)
      }
    })

    it('loads via tn.use()', () => {
      const tn = new Tabnas()
      assert.equal(typeof tn.railroad, 'undefined')
      tn.use(railroadPlugin)
      assert.equal(typeof tn.railroad, 'function')
    })

    it('child instances (tn.make) inherit the decoration', () => {
      const tn = new Tabnas({ plugins: [railroadPlugin] })
      assert.equal(typeof tn.make().railroad, 'function')
    })

    it('exposes the constructors on the installed member', () => {
      const tn = new Tabnas({ plugins: [railroadPlugin] })
      for (const k of ['Diagram', 'seq', 'choice', 'opt', 'plus', 'star', 't', 'n']) {
        assert.equal(typeof tn.railroad[k], 'function', `missing tn.railroad.${k}`)
      }
    })

  })


  describe('text emitter', () => {
    it('quotes terminals and names nonterminals', () => {
      assert.equal(toText(Terminal('hi')), '"hi"')
      assert.equal(toText(NonTerminal('expr')), 'expr')
    })
    it('renders sequence, choice, optional, repetition', () => {
      assert.equal(toText(Sequence('a', 'b')), '"a" "b"')
      assert.equal(toText(Choice('a', 'b')), '("a" | "b")')
      assert.equal(toText(Optional('a')), '["a"]')
      assert.equal(toText(OneOrMore('a')), '"a"+')
      assert.equal(toText(ZeroOrMore('a')), '{"a"}')
    })
  })


  describe('svg node renderer', () => {
    it('emits a well-formed standalone svg', () => {
      const svg = renderNodeSvg(Diagram(Sequence(Terminal('GET'), NonTerminal('path'))))
      assert.match(svg, /^<svg /)
      assert.match(svg, /<\/svg>$/)
      const w = Number(svg.match(/width="(\d+)"/)[1])
      const h = Number(svg.match(/height="(\d+)"/)[1])
      assert.ok(w > 0 && h > 0)
      assert.ok(svg.includes('GET') && svg.includes('path') && svg.includes('<rect'))
    })
    it('renders nested choice/optional/repeat without error', () => {
      const node = Diagram(Sequence(
        Terminal('['),
        Optional(Sequence(NonTerminal('item'), ZeroOrMore(Sequence(Terminal(','), NonTerminal('item'))))),
        Terminal(']')))
      assert.match(renderNodeSvg(node), /^<svg /)
    })
  })


  describe('ascii node renderer', () => {
    it('renders a sequence with box-drawing', () => {
      const out = renderNodeAscii(Diagram(Sequence(Terminal('a'), NonTerminal('b'))))
      assert.ok(out.includes('"a"') && out.includes('b'))
    })
    it('plain mode uses only ASCII bytes', () => {
      const out = renderNodeAscii(Choice(Terminal('a'), Terminal('b')), { ascii: true })
      // eslint-disable-next-line no-control-regex
      assert.ok(/^[\x00-\x7F]*$/.test(out), 'expected pure ASCII output')
    })
  })


  describe('errors', () => {
    it('Choice with no branches throws RailroadError', () => {
      assert.throws(() => Choice(), RailroadError)
    })
    it('rendering an unknown node kind throws RailroadError', () => {
      assert.throws(() => renderNodeSvg({ kind: 'bogus' }), RailroadError)
      assert.throws(() => toText({ kind: 'bogus' }), RailroadError)
    })
    it('an invalid node value throws RailroadError', () => {
      assert.throws(() => Sequence(null), RailroadError)
    })
  })

})
