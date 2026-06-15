/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  railroad.ts
 *  Railroad-diagram plugin for the tabnas parser. Loading it decorates the
 *  instance with `tn.railroad`: a callable that introspects THIS instance's
 *  installed grammar and returns a declarative GrammarModel, plus helpers
 *  to render that grammar to a vertical-flow SVG or ASCII diagram.
 *
 *    const tn = new Tabnas({ plugins: [json, railroad] })
 *    tn.railroad()            // GrammarModel (also tn.railroad.toJson())
 *    tn.railroad.toSvg()      // whole-grammar SVG
 *    tn.railroad.toAscii()    // whole-grammar ASCII
 *
 *  The extraction + rendering logic lives in extract.ts / svg.ts / ascii.ts;
 *  this file is the plugin wiring plus bare exports for instance-free use.
 */

import type {
  Tabnas,
  Plugin,
} from '@tabnas/parser'

import {
  GrammarModel,
  RailroadNode,
  Item,
  RailroadError,
  Terminal,
  NonTerminal,
  Comment,
  Skip,
  Sequence,
  Choice,
  Optional,
  OneOrMore,
  ZeroOrMore,
  Diagram,
  toText,
} from './model'
import { extractGrammar, ExtractOptions } from './extract'
import { modelToSvg, renderNodeSvg, SvgOptions } from './svg'
import { modelToAscii, renderNodeAscii, AsciiOptions } from './ascii'


// The shape `tn.railroad` takes: callable (extract this instance's grammar)
// plus render helpers and the diagram-model constructors for direct use.
export type RailroadApi = ((opts?: ExtractOptions) => GrammarModel) & {
  toJson: (opts?: ExtractOptions) => GrammarModel
  extract: (opts?: ExtractOptions) => GrammarModel
  toSvg: (opts?: SvgOptions & ExtractOptions) => string
  toAscii: (opts?: AsciiOptions & ExtractOptions) => string
  renderNode: (node: Item, opts?: SvgOptions) => string
  renderNodeAscii: (node: Item, opts?: AsciiOptions) => string
  renderNodeText: (node: Item) => string
  Diagram: typeof Diagram
  seq: typeof Sequence
  choice: typeof Choice
  opt: typeof Optional
  plus: typeof OneOrMore
  star: typeof ZeroOrMore
  t: typeof Terminal
  n: typeof NonTerminal
  comment: typeof Comment
  skip: typeof Skip
}


// Plugin entry point. Decoration is lazy: every helper re-reads the
// instance's current grammar when called, so install order does not matter.
const railroad: Plugin = function railroad(tn: Tabnas, _options?: any): void {
  const fn = ((opts?: ExtractOptions): GrammarModel => extractGrammar(tn, opts)) as RailroadApi
  fn.toJson = (opts?: ExtractOptions): GrammarModel => extractGrammar(tn, opts)
  fn.extract = fn.toJson
  fn.toSvg = (opts?: SvgOptions & ExtractOptions): string =>
    modelToSvg(extractGrammar(tn, opts), opts)
  fn.toAscii = (opts?: AsciiOptions & ExtractOptions): string =>
    modelToAscii(extractGrammar(tn, opts), opts)
  fn.renderNode = (node: Item, opts?: SvgOptions): string => renderNodeSvg(node, opts)
  fn.renderNodeAscii = (node: Item, opts?: AsciiOptions): string => renderNodeAscii(node, opts)
  fn.renderNodeText = (node: Item): string => toText(node)
  fn.Diagram = Diagram
  fn.seq = Sequence
  fn.choice = Choice
  fn.opt = Optional
  fn.plus = OneOrMore
  fn.star = ZeroOrMore
  fn.t = Terminal
  fn.n = NonTerminal
  fn.comment = Comment
  fn.skip = Skip
  tn.railroad = fn
}


export {
  railroad,
  extractGrammar,
  modelToSvg,
  modelToAscii,
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
  Comment,
  Skip,
  RailroadError,
}

export type { RailroadNode, GrammarModel }
