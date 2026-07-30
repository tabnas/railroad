/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */
'use strict'

// Cross-runtime conformance, driven by the shared `test/spec/*.tsv` fixtures
// at the repo root — the same convention @tabnas/parser and @tabnas/abnf use
// (see ../../test/AGENTS.md).
//
// `go/parity_test.go` discovers and runs the SAME files, so the two
// implementations cannot drift without one of them going red.

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { toText, renderNodeAscii } = require('..')

const specDir = Path.join(__dirname, '..', '..', 'test', 'spec')

// What the second column holds, keyed by its header name: the renderer to
// run over the node in the first column.
const RENDERERS = {
  text: (node) => toText(node),
  ascii: (node, opts) => renderNodeAscii(node, opts),
}

// Decode the escape set used in non-JSON columns. Kept byte-identical to the
// Go loader so both runtimes see the exact same node source.
function unescape(s) {
  if (!s.includes('\\')) return s
  let out = ''
  for (let i = 0; i < s.length; i++) {
    const c = s[i]
    if ('\\' === c && i + 1 < s.length) {
      const n = s[i + 1]
      if ('n' === n) { out += '\n'; i++; continue }
      if ('r' === n) { out += '\r'; i++; continue }
      if ('t' === n) { out += '\t'; i++; continue }
      if ('\\' === n) { out += '\\'; i++; continue }
    }
    out += c
  }
  return out
}

function loadSpec(file) {
  const body = Fs.readFileSync(Path.join(specDir, file), 'utf8')
  const lines = body.split(/\r?\n/)
  const header = (lines[0] ?? '').split('\t')
  const kind = header[1]
  if (!RENDERERS[kind]) {
    throw new Error(`${file}: unknown second column ${JSON.stringify(kind)}`)
  }
  const rows = []
  for (let i = 1; i < lines.length; i++) {
    const raw = lines[i]
    // A comment line starts with '#' and has no tab; a data row always has
    // at least one (input + expected), so '#'-leading sources still work.
    if ('' === raw || (raw.startsWith('#') && !raw.includes('\t'))) continue
    const cols = raw.split('\t')
    if (cols.length < 2) {
      throw new Error(`${file}:${i + 1}: expected at least 2 tab-separated columns`)
    }
    rows.push({
      line: i + 1,
      node: unescape(cols[0]),
      expected: cols[1],
      opts: cols[2] ?? '',
    })
  }
  return { kind, rows }
}

function label(s) {
  return 60 < s.length ? s.slice(0, 57) + '...' : s
}

function runSpec(file) {
  const { kind, rows } = loadSpec(file)
  describe('spec: ' + file, () => {
    assert.ok(0 < rows.length, file + ': no cases')
    const render = RENDERERS[kind]
    for (const row of rows) {
      it(`row ${row.line}: ${label(row.node)}`, () => {
        // The node column is the node's own JSON shape, which is what both
        // runtimes marshal to and unmarshal from.
        const node = JSON.parse(row.node)
        const opts = '' === row.opts.trim() ? undefined : JSON.parse(row.opts)

        if (row.expected.startsWith('ERROR')) {
          const want = row.expected.slice('ERROR'.length).replace(/^:/, '')
          assert.throws(
            () => render(node, opts),
            (err) => '' === want || err.message.includes(want),
            `${file}:${row.line}: expected ${row.expected}`,
          )
          return
        }

        assert.strictEqual(render(node, opts), JSON.parse(row.expected),
          `${file}:${row.line}`)
      })
    }
  })
}

// Auto-discover every fixture: adding a .tsv runs it in both runtimes
// without touching either runner.
for (const file of Fs.readdirSync(specDir).sort()) {
  if (file.endsWith('.tsv')) runSpec(file)
}
