/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  tabnas-railroad-cli.ts
 *  CLI for the railroad-diagram renderer. Two modes:
 *
 *   grammar mode:  --grammar <module>[#export]
 *     require the module, load its grammar plugin onto a fresh Tabnas
 *     instance, introspect the grammar, and write grammar.railroad.json +
 *     grammar.svg + grammar.txt into the output dir (default ./out).
 *
 *   render mode:   -f <model.json> | -
 *     read a saved GrammarModel and render it to SVG / ASCII / text.
 */

import Fs from 'node:fs'
import Path from 'node:path'

import { Tabnas } from '@tabnas/parser'
import { extractGrammar } from '../extract'
import { modelToSvg } from '../svg'
import { modelToAscii } from '../ascii'
import { toText, GrammarModel } from '../model'


export async function run(argv: string[], console: Console) {
  const args = {
    help: false,
    stdin: false,
    grammar: undefined as string | undefined,
    file: undefined as string | undefined,
    out: undefined as string | undefined,
    svg: false,
    ascii: false,
    json: false,
    text: false,
    plain: false,
  }

  for (let aI = 2; aI < argv.length; aI++) {
    const arg = argv[aI]
    if ('-' === arg) args.stdin = true
    else if ('--help' === arg || '-h' === arg) args.help = true
    else if ('--grammar' === arg || '-g' === arg) args.grammar = argv[++aI]
    else if ('--file' === arg || '-f' === arg) args.file = argv[++aI]
    else if ('--out' === arg || '-o' === arg) args.out = argv[++aI]
    else if ('--svg' === arg) args.svg = true
    else if ('--ascii' === arg) args.ascii = true
    else if ('--json' === arg) args.json = true
    else if ('--text' === arg) args.text = true
    else if ('--ascii-plain' === arg) { args.ascii = true; args.plain = true }
  }

  if (args.help) return help(console)

  // Obtain a GrammarModel.
  let model: GrammarModel
  if (args.grammar) {
    try {
      model = grammarFromModule(args.grammar)
    } catch (e: any) {
      console.error(`tabnas-railroad: ${(e?.message || String(e)).split('\n')[0]}`)
      process.exitCode = 1
      return
    }
  } else if (args.file || args.stdin) {
    let src = ''
    if (args.file) src = Fs.readFileSync(args.file).toString()
    if ('' === src.trim() || args.stdin) src += await readStdin(console)
    try {
      model = JSON.parse(src)
    } catch (e: any) {
      console.error(`tabnas-railroad: invalid JSON grammar model: ${e.message}`)
      process.exitCode = 1
      return
    }
  } else {
    return help(console)
  }

  // grammar mode (or any mode with -o) writes the three artifacts.
  if (args.grammar || args.out) {
    const dir = args.out || 'out'
    const want = anyFormat(args) ? args : { ...args, svg: true, ascii: true, json: true }
    const written: string[] = []
    Fs.mkdirSync(dir, { recursive: true })
    if (want.json) {
      Fs.writeFileSync(Path.join(dir, 'grammar.railroad.json'), JSON.stringify(model, null, 2))
      written.push('grammar.railroad.json')
    }
    if (want.svg) {
      Fs.writeFileSync(Path.join(dir, 'grammar.svg'), modelToSvg(model))
      written.push('grammar.svg')
    }
    if (want.ascii || want.text) {
      const out = want.text ? textModel(model) : modelToAscii(model, { ascii: args.plain })
      Fs.writeFileSync(Path.join(dir, 'grammar.txt'), out)
      written.push('grammar.txt')
    }
    console.log(`wrote ${written.join(', ')} to ${dir}/`)
    return
  }

  // render mode -> stdout, single format (default SVG).
  if (args.json) console.log(JSON.stringify(model, null, 2))
  else if (args.text) console.log(textModel(model))
  else if (args.ascii) console.log(modelToAscii(model, { ascii: args.plain }))
  else console.log(modelToSvg(model))
}


function grammarFromModule(spec: string): GrammarModel {
  const [moduleSpec, exportName] = spec.split('#')
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const mod = require(moduleSpec)
  const plugin = pickPlugin(mod, exportName, moduleSpec)
  if ('function' !== typeof plugin) {
    throw new Error(`could not find a plugin export in '${moduleSpec}'` +
      (exportName ? ` (export '${exportName}')` : ''))
  }
  const tn = new Tabnas({ plugins: [plugin] })
  return extractGrammar(tn)
}

function pickPlugin(mod: any, exportName: string | undefined, moduleSpec: string): any {
  if (exportName && mod[exportName]) return mod[exportName]
  const base = moduleSpec.replace(/^@[^/]+\//, '').replace(/.*\//, '')
  if (mod[base]) return mod[base]
  if ('function' === typeof mod) return mod
  if (mod.default) return mod.default
  for (const k of Object.keys(mod)) if ('function' === typeof mod[k]) return mod[k]
  return null
}

function anyFormat(a: { svg: boolean; ascii: boolean; json: boolean; text: boolean }): boolean {
  return a.svg || a.ascii || a.json || a.text
}

// Compact per-rule EBNF-ish text: `name = <node text>`.
function textModel(model: GrammarModel): string {
  const names = model.start && model.rules[model.start]
    ? [model.start, ...Object.keys(model.rules).filter((n) => n !== model.start)]
    : Object.keys(model.rules)
  return names.map((n) => `${n} = ${toText(model.rules[n])}`).join('\n')
}


async function readStdin(console: Console): Promise<string> {
  if ('string' === typeof (console as any).test$) return (console as any).test$
  if (process.stdin.isTTY) return ''
  let s = ''
  process.stdin.setEncoding('utf8')
  for await (const p of process.stdin) s += p
  return s
}


function help(console: Console) {
  console.log(`
tabnas-railroad: render railroad (syntax) diagrams from a tabnas grammar.

Usage:
  tabnas-railroad --grammar <module>[#export] [-o <dir>] [formats]
  tabnas-railroad -f <model.json> [format]
  echo '<model.json>' | tabnas-railroad - [format]

Modes:
  --grammar <m>[#export]   Load grammar plugin <m> onto a fresh Tabnas
  -g <m>[#export]            instance, introspect it, and write artifacts.
                            e.g. --grammar @tabnas/json
  --file <path>            Render a saved GrammarModel JSON file.
  -f <path>
  -                        Read a GrammarModel JSON from stdin.

Output:
  -o <dir>                 Write grammar.railroad.json + grammar.svg +
                            grammar.txt into <dir> (default ./out in
                            grammar mode). Without -o, render mode prints
                            one format to stdout.

Formats (default: all three when writing, SVG to stdout):
  --json                   Declarative JSON model.
  --svg                    Vertical-flow SVG.
  --ascii                  Vertical ASCII diagram.
  --ascii-plain            ASCII with plain | - + glyphs (implies --ascii).
  --text                   Compact per-rule EBNF text.

  --help, -h               Print this help.

Examples:
  > tabnas-railroad --grammar @tabnas/json -o diagrams
  > tabnas-railroad -f diagrams/grammar.railroad.json --ascii
  > tabnas-railroad --grammar @tabnas/json --text -o /tmp/rr
`)
}
