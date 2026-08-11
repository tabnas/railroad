/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */
'use strict'

// Cross-runtime conformance, driven by the shared `test/spec/*.tsv` fixtures
// at the repo root (see ../../test/AGENTS.md).
//
// The fixture loader, the escape codec, the `ERROR:` contract and the row
// loop all come from @tabnas/support, whose Go half `go/parity_test.go`
// uses to run the SAME files — so the two implementations cannot drift
// without one of them going red, and neither can the two loaders.
//
// What is left here is only what is specific to railroad: which renderer a
// fixture is for.

const { findSpecDir, loadSpecDir, makeRunner } = require('@tabnas/support')

const { toText, renderNodeAscii } = require('..')

// What the second column holds, keyed by its header name: the renderer to
// run over the node in the first column. This is why there is a runner per
// file rather than one over the directory.
const RENDERERS = {
  text: (node) => toText(node),
  ascii: (node, opts) => renderNodeAscii(node, opts),
}

for (const spec of loadSpecDir(findSpecDir(__dirname))) {
  const kind = spec.header[1]
  const render = RENDERERS[kind]
  if (!render) {
    throw new Error(
      `${spec.file}: unknown second column ${JSON.stringify(kind)}`)
  }

  makeRunner({
    // The first column is the node's own JSON shape, which is what both
    // runtimes marshal to and unmarshal from. The third is renderer
    // options, when the renderer takes any.
    parse: (input, row) => {
      const opts = row.named('opts')
      return render(JSON.parse(input), '' === opts.trim()
        ? undefined
        : JSON.parse(opts))
    },

    // The rendered text is compared against the expected column, which
    // holds it as a JSON string — so the comparison is the runner's
    // ordinary one, over two strings.
    expected: kind,
  }).spec(spec)
}
