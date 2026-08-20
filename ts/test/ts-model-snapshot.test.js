/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

// go/testdata/ts-json-model.json is the snapshot of THIS port's model
// that the Go suite compares itself against
// (go/parity_model_test.go, TestParityWithTypeScriptModel).
//
// Nothing on this side read it. A repo-wide search found no TypeScript
// reader and no generator check, so the snapshot was anchored to
// nothing: change the extractor here and the Go test would go on
// comparing against a stale file, with both suites green and the two
// models actually different.
//
// That is the same shape as the defect the Go test was written to fix —
// a golden with no reader — one level up. So this side reads it too, and
// the pair is only meaningful together: Go compares itself to the
// snapshot, and this compares the snapshot to the live model.

const { describe, it } = require('node:test')
const assert = require('node:assert')
const Fs = require('node:fs')
const Path = require('node:path')

const { Tabnas } = require('@tabnas/parser')
const { json } = require('@tabnas/json')
const { railroad: railroadPlugin } = require('..')

const GOLDEN = Path.resolve(
  __dirname, '..', '..', 'go', 'testdata', 'ts-json-model.json')

describe('the Go suite\'s snapshot of this model', () => {

  it('still matches the model this port produces', () => {
    const live = JSON.parse(JSON.stringify(
      new Tabnas({ plugins: [json, railroadPlugin] }).railroad.toJson()))
    const golden = JSON.parse(Fs.readFileSync(GOLDEN, 'utf8'))

    // Sanity first: an empty golden would make every assertion below
    // pass over nothing.
    assert.ok(golden.rules && 0 < Object.keys(golden.rules).length,
      'the snapshot holds no rules, so comparing against it proves nothing')

    // The documented contract, per go/doc/concepts.md: same start, same
    // per-rule node trees, same legend, same ignored set, same
    // meta.engine. Not the whole meta — that map is open-ended.
    assert.deepStrictEqual(live.start, golden.start, 'start drifted')
    assert.deepStrictEqual(live.legend, golden.legend, 'legend drifted')
    assert.deepStrictEqual(live.ignored, golden.ignored, 'ignored drifted')
    assert.deepStrictEqual(live.meta?.engine, golden.meta?.engine,
      'meta.engine drifted')

    assert.deepStrictEqual(
      Object.keys(live.rules).sort(), Object.keys(golden.rules).sort(),
      'rule names drifted')
    for (const name of Object.keys(golden.rules).sort()) {
      assert.deepStrictEqual(live.rules[name], golden.rules[name],
        `rule ${name} drifted — regenerate go/testdata/ts-json-model.json`)
    }
  })
})
